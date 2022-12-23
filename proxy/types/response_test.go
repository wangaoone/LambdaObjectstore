package types

import (
	"context"
	"crypto/rand"
	"io"
	sysnet "net"
	"time"

	"github.com/mason-leap-lab/infinicache/common/net"
	protocol "github.com/mason-leap-lab/infinicache/common/types"
	"github.com/mason-leap-lab/infinicache/common/util"
	"github.com/mason-leap-lab/redeo/resp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func getShortCut() (*net.MockConn, *net.ShortcutConn) {
	shortcut := net.Shortcut.Prepare("localhost", 0, 1)
	return shortcut.Validate(0).Conns[0], shortcut
}

func clearConn(shortcut *net.ShortcutConn) {
	net.Shortcut.Invalidate(shortcut)
}

func readHeader(reader resp.ResponseReader) (chunk string) {
	reader.ReadInt()        // seq
	reader.ReadBulkString() // reqId
	reader.ReadBulkString() // size
	chunk, _ = reader.ReadBulkString()
	return
}

func readBody(reader io.Reader, size int64) (err error) {
	buff := make([]byte, size)
	_, err = reader.Read(buff)
	return
}

func shouldNotTimeout(test func() interface{}, expects ...bool) interface{} {
	expect := false
	if len(expects) > 0 {
		expect = expects[0]
	}

	timer := time.NewTimer(time.Second)
	timeout := false
	responeded := make(chan interface{})
	var ret interface{}
	go func() {
		responeded <- test()
	}()
	select {
	case <-timer.C:
		timeout = true
	case ret = <-responeded:
		if !timer.Stop() {
			<-timer.C
		}
	}

	Expect(timeout).To(Equal(expect))
	return ret
}

// func shouldTimeout(test func() interface{}) {
// 	shouldNotTimeout(test, true)
// }

type holdableInlineReader struct {
	*resp.InlineReader
	util.Closer
}

func (r *holdableInlineReader) Hold() {
	r.Closer.Init()
}

func (r *holdableInlineReader) Unhold() {
	r.Closer.Close()
}

func (r *holdableInlineReader) Close() error {
	r.Closer.Wait()
	return r.InlineReader.Close()
}

var _ = Describe("RequestCoordinator", func() {
	net.InitShortcut()

	It("should WaitFlush return error if cancel before preparing", func() {
		testStream := make([]byte, 1024*1024)
		rand.Read(testStream)
		mock, shortcut := getShortCut()
		defer clearConn(shortcut)

		response := NewResponse(protocol.CMD_GET)
		response.Id.ChunkId = "0"
		stream := &holdableInlineReader{InlineReader: resp.NewInlineReader(testStream)}
		stream.Hold()
		response.SetBodyStream(stream)

		// Simulate cancel before preparing
		response.CancelFlush()

		response.PrepareForGet(resp.NewResponseWriter(mock.Server), 0)

		chErr := make(chan error)
		go func() {
			chErr <- response.Flush()
		}()

		err := response.waitFlush(true, func() sysnet.Conn { return mock.Server })
		Expect(err).To(Equal(context.Canceled))
		Expect(response.Context().Err()).To(Equal(context.Canceled))
		Expect(mock.Server.Status()).To(Equal("")) // Not disconnected

		chunkId := readHeader(resp.NewResponseReader(mock.Client))
		Expect(chunkId).To(Equal("-1"))

		Expect(<-chErr).To(BeNil())
		shouldNotTimeout(func() interface{} {
			response.Close()
			return nil
		})
	})

	It("should WaitFlush return error if cancel before transmission", func() {
		testStream := make([]byte, 1024*1024)
		rand.Read(testStream)
		mock, shortcut := getShortCut()
		defer clearConn(shortcut)

		response := NewResponse(protocol.CMD_GET)
		response.Id.ChunkId = "0"
		stream := &holdableInlineReader{InlineReader: resp.NewInlineReader(testStream)}
		response.SetBodyStream(stream)
		response.PrepareForGet(resp.NewResponseWriter(mock.Server), 0)

		chErr := make(chan error)
		go func() {
			chErr <- response.Flush()
		}()

		chunkId := readHeader(resp.NewResponseReader(mock.Client))
		Expect(chunkId).To(Equal("0"))

		// Simulate cancel behavior
		response.CancelFlush()

		err := response.waitFlush(true, func() sysnet.Conn { return mock.Server })
		Expect(err).To(Equal(context.Canceled))
		Expect(response.Context().Err()).To(Equal(context.Canceled))
		Expect(mock.Server.Status()).To(Equal("closedAbandon"))

		err = readBody(mock.Client, int64(len(testStream)/2)) // Read some
		Expect(err).To(Equal(io.EOF))

		Expect(<-chErr).To(Equal(context.Canceled))
		shouldNotTimeout(func() interface{} {
			response.Close()
			return nil
		})
	})

	It("should WaitFlush return error if cancel during transmission", func() {
		testStream := make([]byte, 1024*1024)
		rand.Read(testStream)
		mock, shortcut := getShortCut()
		defer clearConn(shortcut)

		response := NewResponse(protocol.CMD_GET)
		response.Id.ChunkId = "0"
		stream := &holdableInlineReader{InlineReader: resp.NewInlineReader(testStream)}
		response.SetBodyStream(stream)
		response.PrepareForGet(resp.NewResponseWriter(mock.Server), 0)

		chErr := make(chan error)
		go func() {
			chErr <- response.Flush()
		}()

		chunkId := readHeader(resp.NewResponseReader(mock.Client))
		Expect(chunkId).To(Equal("0"))
		readBody(mock.Client, int64(len(testStream)/2)) // Half read

		// Simulate cancel behavior
		response.CancelFlush()

		err := response.waitFlush(true, func() sysnet.Conn { return mock.Server })
		Expect(err).To(Equal(context.Canceled))
		Expect(response.Context().Err()).To(Equal(context.Canceled))
		Expect(mock.Server.Status()).To(Equal("closedAbandon"))

		err = readBody(mock.Client, int64(len(testStream)/2)) // Read rest
		Expect(err).To(Equal(io.EOF))

		Expect(<-chErr).To(Equal(context.Canceled))
		shouldNotTimeout(func() interface{} {
			response.Close()
			return nil
		})
	})

	It("should WaitFlush return success if cancel after transmission", func() {
		testStream := make([]byte, 1024*1024)
		rand.Read(testStream)
		mock, shortcut := getShortCut()
		defer clearConn(shortcut)

		response := NewResponse(protocol.CMD_GET)
		response.Id.ChunkId = "0"
		stream := &holdableInlineReader{InlineReader: resp.NewInlineReader(testStream)}
		response.SetBodyStream(stream)
		response.PrepareForGet(resp.NewResponseWriter(mock.Server), 0)

		chErr := make(chan error)
		go func() {
			chErr <- response.Flush()
		}()

		reader := resp.NewResponseReader(mock.Client)
		chunkId := readHeader(reader)
		Expect(chunkId).To(Equal("0"))
		allReader, _ := reader.StreamBulk()
		_, err := allReader.ReadAll()
		Expect(err).To(BeNil())

		// Simulate cancel before preparing
		go response.CancelFlush()

		err = response.waitFlush(true, func() sysnet.Conn { return mock.Server })
		Expect(err).To(BeNil())
		Expect(response.Context().Err()).To(Equal(context.Canceled))
		Expect(mock.Server.Status()).To(Equal("")) // Not disconnected

		Expect(<-chErr).To(BeNil())
		shouldNotTimeout(func() interface{} {
			response.Close()
			return nil
		})
	})
})
