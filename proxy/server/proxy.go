package server

import (
	"github.com/google/uuid"

	//	"github.com/google/uuid"
	"github.com/mason-leap-lab/infinicache/common/logger"
	"github.com/mason-leap-lab/infinicache/common/util"
	"github.com/mason-leap-lab/redeo"
	"github.com/mason-leap-lab/redeo/resp"
	"net"
	"math/rand"
	"strconv"
	"sync"
	"time"

	protocol "github.com/mason-leap-lab/infinicache/common/types"
	"github.com/mason-leap-lab/infinicache/proxy/collector"
	"github.com/mason-leap-lab/infinicache/proxy/global"
	"github.com/mason-leap-lab/infinicache/proxy/lambdastore"
	"github.com/mason-leap-lab/infinicache/proxy/types"
)

type Proxy struct {
	log       logger.ILogger
	group     *Group
	metaStore *Placer

	initialized int32
	ready       sync.WaitGroup
}

// initial lambda group
func New(replica bool) *Proxy {
	group := NewGroup(NumLambdaClusters)
	p := &Proxy{
		log: &logger.ColorLogger{
			Prefix: "Proxy ",
			Level:  global.Log.GetLevel(),
			Color:  true,
		},
		group:     group,
		metaStore: NewPlacer(NewMataStore(), group),
	}

	for i := range p.group.All {
		name := LambdaPrefix
		if replica {
			p.log.Info("[Registering lambda store replica %d.]", i)
			name = LambdaStoreName
		} else {
			p.log.Info("[Registering lambda store %s%d]", name, i)
		}
		node := scheduler.GetForGroup(p.group, i)
		node.Meta.Capacity = InstanceCapacity
		node.Meta.IncreaseSize(InstanceOverhead)
	}
	// Something can only be done after all nodes initialized.
	for i := range p.group.All {
		num, candidates := p.getBackupsForNode(p.group, i)
		node := p.group.Instance(i)
		node.AssignBackups(num, candidates)

		// Initialize instance, this is not neccessary if the start time of the instance is acceptable.
		p.ready.Add(1)
		go func() {
			node.WarmUp()
			p.ready.Done()
		}()

		// Begin handle requests
		go node.HandleRequests()
	}

	return p
}

func (p *Proxy) Serve(lis net.Listener) {
	for {
		cn, err := lis.Accept()
		if err != nil {
			return
		}

		conn := lambdastore.NewConnection(cn)
		go conn.ServeLambda()
	}
}

func (p *Proxy) WaitReady() {
	p.ready.Wait()
	p.log.Info("[Proxy is ready]")
}

func (p *Proxy) Close(lis net.Listener) {
	lis.Close()
}

func (p *Proxy) Release() {
	for i, node := range p.group.All {
		scheduler.Recycle(node.LambdaDeployment)
		p.group.All[i] = nil
	}
	scheduler.Clear(p.group)
}

// from client
func (p *Proxy) HandleSet(w resp.ResponseWriter, c *resp.CommandStream) {
	client := redeo.GetClient(c.Context())
	connId := int(client.ID())

	// Get args
	key, _ := c.NextArg().String()
	dChunkId, _ := c.NextArg().Int()
	chunkId := strconv.FormatInt(dChunkId, 10)
	lambdaId, _ := c.NextArg().Int()
	randBase, _ := c.NextArg().Int()
	reqId, _ := c.NextArg().String()
	// _, _ = c.NextArg().Int()
	// _, _ = c.NextArg().Int()
	dataChunks, _ := c.NextArg().Int()
	parityChunks, _ := c.NextArg().Int()

	bodyStream, err := c.Next()
	if err != nil {
		p.log.Error("Error on get value reader: %v", err)
		return
	}
	bodyStream.(resp.Holdable).Hold()

	// Start counting time.
	if err := collector.Collect(collector.LogStart, "set", reqId, chunkId, time.Now().UnixNano()); err != nil {
		p.log.Warn("Fail to record start of request: %v", err)
	}

	// We don't use this for now
	// global.ReqMap.GetOrInsert(reqId, &types.ClientReqCounter{"set", int(dataChunks), int(parityChunks), 0})

	// Check if the chunk key(key + chunkId) exists, base of slice will only be calculated once.
	prepared := p.metaStore.NewMeta(
		key, int(randBase), int(dataChunks+parityChunks), int(dChunkId), int(lambdaId), bodyStream.Len())

	meta, _, postProcess := p.metaStore.GetOrInsert(key, prepared)
	if meta.Deleted {
		// Object may be evicted in somecase:
		// 1: Some chunks were set.
		// 2: Placer evicted this object (unlikely).
		// 3: We got evicted meta.
		p.log.Warn("KEY %s@%s not set to lambda store, may got evicted before all chunks are set.", chunkId, key)
		w.AppendErrorf("KEY %s@%s not set to lambda store, may got evicted before all chunks are set.", chunkId, key)
		w.Flush()
		return
	}
	if postProcess != nil {
		postProcess(p.dropEvicted)
	}
	chunkKey := meta.ChunkKey(int(dChunkId))
	lambdaDest := meta.Placement[dChunkId]

	// Send chunk to the corresponding lambda instance in group
	p.log.Debug("Requesting to set %s: %d", chunkKey, lambdaDest)
	p.group.Instance(lambdaDest).C() <- &types.Request{
		Id:           types.Id{connId, reqId, chunkId},
		InsId:        uint64(lambdaDest),
		Cmd:          protocol.CMD_SET,
		Key:          chunkKey,
		BodyStream:   bodyStream,
		ChanResponse: client.Responses(),
		EnableCollector: true,
	}
	// p.log.Debug("KEY is", key.String(), "IN SET UPDATE, reqId is", reqId, "connId is", connId, "chunkId is", chunkId, "lambdaStore Id is", lambdaId)
}

func (p *Proxy) HandleGet(w resp.ResponseWriter, c *resp.Command) {
	client := redeo.GetClient(c.Context())
	connId := int(client.ID())
	key := c.Arg(0).String()
	dChunkId, _ := c.Arg(1).Int()
	chunkId := strconv.FormatInt(dChunkId, 10)
	reqId := c.Arg(2).String()
	dataChunks, _ := c.Arg(3).Int()
	parityChunks, _ := c.Arg(4).Int()

	// Start couting time.
	if err := collector.Collect(collector.LogStart, "get", reqId, chunkId, time.Now().UnixNano()); err != nil {
		p.log.Warn("Fail to record start of request: %v", err)
	}

	counter := global.ReqCoordinator.Register(reqId, protocol.CMD_GET, dataChunks, parityChunks)

	// key is "key"+"chunkId"
	meta, ok := p.metaStore.Get(key, int(dChunkId))
	if !ok || meta.Deleted {
		// Object may be deleted.
		p.log.Warn("KEY %s@%s not found in lambda store, please set first.", chunkId, key)
		w.AppendErrorf("KEY %s@%s not found in lambda store, please set first.", chunkId, key)
		w.Flush()
		return
	}
	chunkKey := meta.ChunkKey(int(dChunkId))
	lambdaDest := meta.Placement[dChunkId]

	// Send request to lambda channel
	p.log.Debug("Requesting to get %s: %d", chunkKey, lambdaDest)
	req := &types.Request{
		Id:           types.Id{connId, reqId, chunkId},
		InsId:        uint64(lambdaDest),
		Cmd:          protocol.CMD_GET,
		Key:          chunkKey,
		ChanResponse: client.Responses(),
		EnableCollector: true,
	}
	counter.Requests[dChunkId] = req
	// Unlikely, just to be safe
	if counter.IsFulfilled(counter.Returned()) {
		returned := counter.AddReturned(int(dChunkId))
		req.Abandon()
		if counter.IsAllReturned(returned) {
			global.ReqCoordinator.Clear(reqId, counter)
		}
	} else {
		p.group.Instance(lambdaDest).C() <- req
	}
}

func (p *Proxy) HandleCallback(w resp.ResponseWriter, r interface{}) {
	wrapper := r.(*types.ProxyResponse)
	switch rsp := wrapper.Response.(type) {
	case *types.Response:
		t := time.Now()

		rsp.PrepareFor(w)
		d1 := time.Since(t)

		t2 := time.Now()
		// flush buffer, return on errors
		if err := rsp.Flush(); err != nil {
			p.log.Error("Error on flush response: %v", err)
			return
		}
		d2 := time.Since(t2)
		//p.log.Debug("Server AppendInt time is", time0,
		//	"AppendBulk time is", time1,
		//	"Server Flush time is", time2,
		//	"Chunk body len is ", len(rsp.Body))
		tgg := time.Now()
		if wrapper.Request.EnableCollector {
			err := collector.Collect(collector.LogServer2Client, rsp.Cmd, rsp.Id.ReqId, rsp.Id.ChunkId, int64(tgg.Sub(t)), int64(d1), int64(d2), tgg.UnixNano())
			if err != nil {
				p.log.Warn("LogServer2Client err %v", err)
			}
		}
	// Use more general way to deal error
	default:
		w.AppendErrorf("%v", rsp)
		w.Flush()
	}
}

func (p *Proxy) CollectData() {
	for i, _ := range p.group.All {
		global.DataCollected.Add(1)
		// send data command
		p.group.Instance(i).C() <- &types.Control{Cmd: "data"}
	}
	p.log.Info("Waiting data from Lambda")
	global.DataCollected.Wait()
	if err := collector.Flush(); err != nil {
		p.log.Error("Failed to save data from lambdas: %v", err)
	} else {
		p.log.Info("Data collected.")
	}
}

func (p *Proxy) getBackupsForNode(g *Group, i int) (int, []*lambdastore.Instance) {
	numBaks := BackupsPerInstance
	numTotal := numBaks * 2
	distance := g.Len() / (numTotal + 1)     // main + double backup candidates
	if distance == 0 {
		// In case 2 * total >= g.Len()
		distance = 1
		numBaks = util.Ifelse(numBaks >= g.Len(), g.Len() - 1, numBaks).(int)    // Use all
		numTotal = util.Ifelse(numTotal >= g.Len(), g.Len() - 1, numTotal).(int)
	}
	candidates := make([]*lambdastore.Instance, numTotal)
	for j := 0; j < numTotal; j++ {
		candidates[j] = g.Instance((i + j * distance + rand.Int() % distance + 1) % g.Len()) // Random to avoid the same backup set.
	}
	return numBaks, candidates
}

func (p *Proxy) dropEvicted(meta *Meta) {
	reqId := uuid.New().String()
	for i, lambdaId := range meta.Placement {
		instance := p.group.Instance(lambdaId)
		instance.C() <- &types.Request{
			Id:    types.Id{0, reqId, strconv.Itoa(i)},
			InsId: uint64(lambdaId),
			Cmd:   protocol.CMD_DEL,
			Key:   meta.ChunkKey(i),
		}
	}
}
