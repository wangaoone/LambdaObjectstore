package lambdastore

import (
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/mason-leap-lab/infinicache/common/logger"
	protocol "github.com/mason-leap-lab/infinicache/common/types"
	"github.com/mason-leap-lab/infinicache/common/util"
	"github.com/mason-leap-lab/infinicache/proxy/collector"
	"github.com/mason-leap-lab/infinicache/proxy/global"
	"github.com/mason-leap-lab/infinicache/proxy/types"
	"github.com/mason-leap-lab/redeo/resp"
)

var (
	defaultConnectionLog = &logger.ColorLogger{
		Prefix: "Undesignated ",
		Color:  !global.Options.NoColor,
	}
	ErrConnectionClosed = errors.New("connection closed")
	ErrMissingResponse  = errors.New("missing response")
	ErrUnexpectedType   = errors.New("unexpected type")
)

type Connection struct {
	net.Conn
	workerId int32       // Identify a unique lambda worker
	control  bool        // Identify if the connection is control link or not.
	dataLink *Connection // Only works for the control link to find its data link.
	ctrlLink *Connection // Only works for the data link to find its control link.

	instance    *Instance
	log         logger.ILogger
	w           *resp.RequestWriter
	r           resp.ResponseReader
	mu          sync.Mutex
	chanWait    chan *types.Request
	headRequest *types.Request
	respType    chan interface{}
	closed      chan struct{}
}

func NewConnection(c net.Conn) *Connection {
	conn := &Connection{
		Conn: c,
		log:  defaultConnectionLog,
		// wrap writer and reader
		w:        resp.NewRequestWriter(c),
		r:        resp.NewResponseReader(c),
		chanWait: make(chan *types.Request, 1),
		respType: make(chan interface{}),
		closed:   make(chan struct{}),
	}
	defaultConnectionLog.Level = global.Log.GetLevel()
	return conn
}

func (conn *Connection) String() string {
	return fmt.Sprintf("%d%s", conn.workerId, util.Ifelse(conn.control, "c", "d"))
}

func (conn *Connection) Writer() *resp.RequestWriter {
	return conn.w
}

func (conn *Connection) BindInstance(ins *Instance) {
	if conn.instance == ins {
		return
	}

	conn.instance = ins
	conn.log = ins.log
	if l, ok := conn.log.(*logger.ColorLogger); ok {
		copied := *l
		copied.Prefix = fmt.Sprintf("%s%v ", copied.Prefix, conn)
		conn.log = &copied
	}
}

func (conn *Connection) GetDataLink() *Connection {
	return conn.dataLink
}

func (conn *Connection) AddDataLink(link *Connection) bool {
	if conn.workerId != link.workerId {
		return false
	}

	link.ctrlLink, conn.dataLink = conn, link
	conn.log.Debug("Data link added:%v", link)
	return true
}

func (conn *Connection) RemoveDataLink(link *Connection) {
	if conn.dataLink != nil && conn.dataLink == link {
		conn.dataLink = nil
	}
}

func (conn *Connection) SendControl(ctrl *types.Control) error {
	if err := ctrl.Flush(); err != nil {
		conn.log.Error("Flush control error: %v - %v", ctrl, err)

		conn.close()

		return err
	}

	return nil
}

func (conn *Connection) SendRequest(req *types.Request) error {
	// In case there is a request already, wait to be consumed (for response).
	// Lock free: request sent after headRequest get set.
	conn.log.Debug("Waiting for sending %v(wait: %d)", req, len(conn.chanWait))
	conn.chanWait <- req
	conn.headRequest = req

	// Updated by Tianium: 20210504
	// Write timeout is meaningless: small data will be buffered and always success, blobs will be handled by both ends.
	// TODO: If neccessary (like add support to multi-layer relay), add read timeout on client.
	if err := req.Flush(); err != nil && req.SetResponse(err) { // If req is responded, err has been reported somewhere.
		conn.log.Warn("Flush request error: %v - %v", req, err)

		// Remove request.
		conn.popRequest()

		// Close connection
		conn.close()

		return err
	}
	// Set timeout for response.
	req.SetTimeout(ResponseTimeout)
	go func() {
		if err := req.Timeout(); err != nil && req.SetResponse(err) { // If req is responded, err has been reported somewhere.
			conn.log.Warn("Request timeout: %v", req)

			// Remove request.
			conn.popRequest()

			// Late response will be ignored.
		}
	}()

	return nil
}

func (conn *Connection) peekRequest() *types.Request {
	return conn.headRequest
}

func (conn *Connection) popRequest() *types.Request {
	conn.headRequest = nil
	select {
	case req := <-conn.chanWait:
		return req
	default:
		return nil
	}
}

func (conn *Connection) isRequestPending() bool {
	return len(conn.chanWait) > 0
}

// Close Signal connection should be closed. Function close() will be called later for actural operation
func (conn *Connection) Close() error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	select {
	case <-conn.closed:
		// already closed
		return nil
	default:
	}

	// Signal closed only. This allow ongoing transmission to finish.
	conn.log.Debug("Signal to close.")
	close(conn.closed)

	// Signal data link to close itself.
	dataLink := conn.dataLink
	if dataLink != nil {
		dataLink.Close()
		conn.dataLink = nil
	}

	return nil
}

func (conn *Connection) close() {
	// Data link: Try detach from control link.
	// Control link: Notify instance.
	if conn.ctrlLink != nil {
		conn.ctrlLink.RemoveDataLink(conn)
		conn.ctrlLink = nil
	} else if conn.instance != nil {
		conn.instance.FlagClosed(conn)
	}

	// Call signal function to avoid duplicated close.
	conn.Close()

	// Don't use conn.Conn.Close(), it will stuck and wait for lambda.
	if tcp, ok := conn.Conn.(*net.TCPConn); ok {
		tcp.SetLinger(0) // The operating system discards any unsent or unacknowledged data.
	}
	conn.Conn.Close()

	// Clear pending requests after TCP connection closed, so current request got chance to return first.
	conn.ClearResponses()

	conn.log.Debug("Closed.")
}

// blocking on lambda peek Type
// lambda handle incoming lambda store response
//
// field 0 : conn id
// field 1 : req id
// field 2 : chunk id
// field 3 : obj val
func (conn *Connection) ServeLambda() {
	for {
		go conn.peekResponse()

		var retPeek interface{}
		select {
		case <-conn.closed:
			if conn.isRequestPending() {
				// Is there request waiting?
				retPeek = <-conn.respType
			} else {
				conn.close()
				return
			}
		case retPeek = <-conn.respType:
		}

		// Got response, reset read deadline.
		conn.Conn.SetReadDeadline(time.Time{})

		var respType resp.ResponseType
		switch ret := retPeek.(type) {
		case error:
			if ret == io.EOF {
				conn.log.Warn("Lambda store disconnected.")
			} else {
				conn.log.Warn("Failed to peek response type: %v", ret)
			}
			conn.close()
			return
		case resp.ResponseType:
			respType = ret
		}

		var readErr error
		start := time.Now()
		switch respType {
		case resp.TypeError:
			strErr, err := conn.r.ReadError()
			if err != nil {
				readErr = err
				break
			}
			err = fmt.Errorf("response error: %s", strErr)
			conn.log.Warn("%v", err)
			conn.SetErrorResponse(err)
		case resp.TypeBulk:
			cmd, err := conn.r.ReadBulkString()
			if err != nil {
				readErr = err
				break
			}

			if cmd == protocol.CMD_PONG {
				conn.pongHandler()
				break
			} else if conn.instance == nil {
				// all other commands require instance association, or could be a rougue request.
				break
			}

			switch cmd {
			case protocol.CMD_RECOVERED:
				conn.recoveredHandler()
			case protocol.CMD_GET:
				conn.getHandler(start)
			case protocol.CMD_SET:
				conn.setHandler(start)
			case protocol.CMD_DEL:
				conn.delHandler()
			case protocol.CMD_DATA:
				conn.receiveData()
			case protocol.CMD_INITMIGRATE:
				go conn.initMigrateHandler()
			case protocol.CMD_BYE:
				conn.bye()
			case protocol.CMD_RECOVER:
				conn.recoverHandler()
			default:
				conn.log.Warn("Unsupported response type: %s", logger.SafeString(cmd, 20))
			}
		default:
			if err := conn.skipField(respType); err != nil {
				readErr = err
			}
		}

		if readErr != nil && readErr == io.EOF {
			conn.log.Warn("Lambda store disconnected.")
			conn.close()
			return
		} else if readErr != nil {
			conn.log.Warn("Error on handle response %s: %v", respType, readErr)
			conn.close()
			return
		}

		if conn.instance != nil {
			conn.instance.flagWarmed()
		}
	}
}

func (conn *Connection) skipField(t resp.ResponseType) error {
	if t == resp.TypeUnknown {
		var err error
		t, err = conn.r.PeekType()
		if err != nil {
			return err
		}
	}

	switch t {
	case resp.TypeArray:
		if arrLen, err := conn.r.ReadArrayLen(); err != nil {
			return err
		} else {
			conn.log.Warn("Skipping %s of len %d", t, arrLen)
			for i := 0; i < arrLen; i++ {
				if err := conn.skipField(resp.TypeUnknown); err != nil {
					return err
				}
			}
		}
	case resp.TypeBulk:
		conn.log.Warn("Skipping %s", t)
		if err := conn.r.SkipBulk(); err != nil {
			return err
		}
	case resp.TypeUnknown:
		return ErrUnexpectedType
	default:
		conn.log.Warn("Skipping %s", t)
		// We just want read a line, ignore other error
		if _, err := conn.r.ReadInlineString(); err != nil && !resp.IsProtocolError(err) {
			return err
		}
	}
	return nil
}

func (conn *Connection) Ping(payload []byte) {
	conn.w.WriteMultiBulkSize(2)
	conn.w.WriteBulkString(protocol.CMD_PING)
	conn.w.WriteBulk(payload)
	conn.SetWriteDeadline(time.Now().Add(RequestTimeout))
	defer conn.SetWriteDeadline(time.Time{})
	err := conn.w.Flush()
	if err != nil {
		conn.log.Warn("Flush ping error: %v", err)
		conn.close()
	}
}

// SetResponse Set response for last request.
func (conn *Connection) SetResponse(rsp *types.Response) (*types.Request, bool) {
	// Last request can be responded, either bacause error or timeout, which causes nil or unmatch
	req := conn.peekRequest()
	if req == nil || !req.IsResponse(rsp) {
		// ignore
		conn.log.Debug("response discarded: %v", rsp)
		return nil, false
	}

	// Lock free: double check that poped is what we peeked.
	if poped := conn.popRequest(); poped == req {
		conn.log.Debug("response matched: %v", req)
		return req, req.SetResponse(rsp)
	}

	return nil, false
}

func (conn *Connection) SetErrorResponse(err error) bool {
	if req := conn.popRequest(); req != nil {
		return req.SetResponse(err)
	}

	conn.log.Warn("Unexpected error response: no request pending. err: %v", err)
	return false
}

func (conn *Connection) ClearResponses() {
	req := conn.popRequest()
	for req != nil {
		req.SetResponse(ErrConnectionClosed)
		// Yield for pending req a chance to push.
		runtime.Gosched()

		req = conn.popRequest()
	}
}

func (conn *Connection) peekResponse() {
	respType, err := conn.r.PeekType()
	if err != nil {
		conn.respType <- err
	} else {
		conn.respType <- respType
	}
}

func (conn *Connection) closeIfError(prompt string, err error) bool {
	if err != nil {
		conn.log.Warn(prompt, err)
		conn.close()
		return true
	}

	return false
}

func (conn *Connection) pongHandler() {
	// Read lambdaId, if it is negatvie, we need a parallel recovery.
	id, err := conn.r.ReadInt()
	if conn.closeIfError("Discard rouge PONG for missing store id: %v.", err) {
		return
	}
	storeId := id & 0xFFFF
	conn.workerId = int32(id >> 32)

	sid, err := conn.r.ReadBulkString()
	if conn.closeIfError("Discard rouge PONG for missing session id: %v.", err) {
		return
	}

	flags, err := conn.r.ReadInt()
	if conn.closeIfError("Discard rouge PONG for missing flags: %v.", err) {
		return
	}
	conn.control = flags&protocol.PONG_FOR_CTRL > 0

	conn.log.Debug("PONG from lambda(%d,flag:%d).", storeId, flags)
	instance := conn.instance
	if instance == nil {
		// Look up lambda instance
		instance = CM.Instance(uint64(storeId))
	}
	if instance == nil {
		conn.log.Error("Failed to match lambda: %d", storeId)
		return
	}

	conn, err = instance.TryFlagValidated(conn, sid, flags)
	if err == nil || err == ErrNotCtrlLink || err == ErrInstanceValidated {
		conn.log.Debug("PONG from lambda confirmed.")
	} else {
		conn.log.Warn("Discard rouge PONG for %d.", storeId)
		conn.close()
	}
}

func (conn *Connection) recoveredHandler() {
	conn.log.Debug("RECOVERED from lambda.")

	if conn.instance == nil {
		conn.log.Error("No instance set on recovered")
		return
	}

	conn.instance.ResumeServing()
}

func (conn *Connection) getHandler(start time.Time) {
	conn.log.Debug("GET from lambda.")

	// Exhaust all values to keep protocol aligned.
	reqId, _ := conn.r.ReadBulkString()
	chunkId, _ := conn.r.ReadBulkString()
	recovered, _ := conn.r.ReadInt()
	counter, _ := global.ReqCoordinator.Load(reqId).(*global.RequestCounter)
	if counter == nil {
		conn.log.Warn("Request not found: %s", reqId)
		// exhaust value field
		if err := conn.r.SkipBulk(); err != nil {
			conn.log.Warn("Failed to skip bulk on request mismatch: %v", err)
		}
		return
	}

	rsp := &types.Response{Cmd: "get"}
	rsp.Id.ReqId = reqId
	rsp.Id.ChunkId = chunkId
	rsp.Status = recovered
	chunk, _ := strconv.Atoi(chunkId)

	status := counter.AddSucceeded(chunk, recovered == 1)
	// Check if chunks are enough? Shortcut response if YES.
	if counter.IsLate(status) {
		conn.log.Debug("GOT %v, abandon.", rsp.Id)
		// Most likely, the req has been abandoned already. But we still need to consume the connection side req.
		req, _ := conn.SetResponse(rsp)
		if req != nil {
			_, err := collector.CollectRequest(collector.LogProxy, req.CollectorEntry, start.UnixNano(), int64(time.Since(start)), int64(0), recovered)
			if err != nil {
				conn.log.Warn("LogProxy err %v", err)
			}
		}
		counter.ReleaseIfAllReturned(status)

		// Consume and abandon the response.
		if err := conn.r.SkipBulk(); err != nil {
			conn.log.Warn("Failed to skip bulk on abandon: %v", err)
		}
		return
	}

	var err error
	rsp.BodyStream, err = conn.r.StreamBulk()
	if err != nil {
		conn.log.Warn("Failed to get body reader of response: %v", err)
	}
	rsp.BodyStream.(resp.Holdable).Hold()
	defer rsp.BodyStream.Close()

	conn.log.Debug("GOT %v, confirmed.", rsp.Id)
	if req, ok := conn.SetResponse(rsp); !ok {
		// Failed to set response, release hold.
		rsp.BodyStream.(resp.Holdable).Unhold()
	} else {
		collector.CollectRequest(collector.LogProxy, req.CollectorEntry, start.UnixNano(), int64(time.Since(start)), int64(0), recovered)
	}
	// Abandon rest chunks.
	if counter.IsFulfilled(status) && !counter.IsAllReturned() { // IsAllReturned will load updated status.
		conn.log.Debug("Request fulfilled: %v, abandon rest chunks.", rsp.Id)
		for _, req := range counter.Requests {
			if req != nil && !req.IsReturnd() {
				// For returned requests, it can be faster one or late one, their connection will decide.
				req.Abandon()
			}
		}
	}
}

func (conn *Connection) setHandler(start time.Time) {
	conn.log.Debug("SET from lambda.")

	rsp := &types.Response{Cmd: "set", Body: []byte(strconv.FormatUint(conn.instance.Id(), 10))}
	rsp.Id.ReqId, _ = conn.r.ReadBulkString()
	rsp.Id.ChunkId, _ = conn.r.ReadBulkString()

	conn.log.Debug("SET %v, confirmed.", rsp.Id)
	req, ok := conn.SetResponse(rsp)
	if ok {
		collector.CollectRequest(collector.LogProxy, req.CollectorEntry, start.UnixNano(), int64(time.Since(start)), int64(0), int64(0))
	}
}

func (conn *Connection) delHandler() {
	conn.log.Debug("DEL from lambda.")

	rsp := &types.Response{Cmd: "del"}

	rsp.Id.ReqId, _ = conn.r.ReadBulkString()
	rsp.Id.ChunkId, _ = conn.r.ReadBulkString()

	conn.log.Debug("DEL %v, confirmed.", rsp.Id)
	conn.SetResponse(rsp) // if del is control cmd, should return False
}

func (conn *Connection) receiveData() {
	conn.log.Debug("DATA from lambda.")

	ok, err := conn.r.ReadBulkString()
	if err != nil {
		conn.log.Error("Error on processing result of data collection: %v", err)
	}
	conn.instance.FlagDataCollected(ok)
}

func (conn *Connection) initMigrateHandler() {
	conn.log.Debug("Request to initiate migration")

	conn.instance.Migrate()
}

func (conn *Connection) bye() {
	conn.log.Debug("BYE from lambda.")
	if conn.instance != nil {
		conn.instance.bye(conn)
	}
}

func (conn *Connection) recoverHandler() {
	conn.log.Debug("RECOVER from lambda.")

	reqId, _ := conn.r.ReadBulkString()
	conn.r.ReadBulkString() // chunkId

	ctrl := global.ReqCoordinator.Load(reqId)
	if ctrl == nil {
		conn.log.Warn("No control found for %s", reqId)
	} else if ctrl.(*types.Control).Callback == nil {
		conn.log.Warn("Control callback not defined for recover request %s", reqId)
	} else {
		ctrl.(*types.Control).Callback(ctrl.(*types.Control), conn.instance)
	}
}
