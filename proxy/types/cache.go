package types

import (
	"errors"
	"time"

	"github.com/mason-leap-lab/redeo"
	"github.com/mason-leap-lab/redeo/resp"
)

var (
	ErrInvalidChunkSize = errors.New("invalid chunk size")
	ErrUnimplemented    = errors.New("unimplemented")
	ErrRequestFailure   = errors.New("request failed")
	ErrChunkClosed      = errors.New("chunk closed")
	ErrChunkStoreFailed = errors.New("failed to cache chunk")

	CtxKeyRequest = cacheCtxKey("request")
)

type cacheCtxKey string

type PersistRetrier func(PersistChunk)

// PersistCache offers API to cache data temporarily for persistent support and request batching.
// For persistent support, chunk to be written will
// 1. Stores in the PersistCache and persists to local storage before writing to SMS.
// 2. Once the chunk is written to SMS, the request will return without waiting for the chunk to be persisted to the COS.
// 3. If persisting to COS fails, the chunk will be loaded from PersistCache and retry persisting again.
// 4. If the chunk is requested before persisting to COS for scaling purpose, the chunk will be served from PersistCache.
// 5. After persisted to COS, the chunk will be removed from PersistCache.
// 6. On proxy failure, all chunks stored in PersistCache will be restored from local storage.
// For request batching, concurrent chunk requests will be merged into one request as:
// 1. The first request will load chunk from SMS and store it in PersistCache.
// 2. The following requests will load chunk from PersistCache.
type PersistCache interface {
	// Len returns the number of chunks in the cache.
	Len() int

	// Get returns a PersistChunk instance by specified key.
	// This call is read-after-write safe because the key(chunk key) will only
	// be available to get request after an earlier write request was finished.
	GetOrCreate(key string, size int64) (chunk PersistChunk, first bool)

	// Get will return a existed PersistChunk, nil if not found.
	Get(key string) PersistChunk

	// Restore restores the cache from local storage.
	Restore() error
}

// PersistChunk offers API for a abstract chunk to support persisting.
type PersistChunk interface {
	redeo.Contextable

	// Key returns the key of the chunk.
	Key() string

	// Size returns the size of the chunk.
	Size() int64

	// IsStored returns whether the chunk is stored.
	IsStored() bool

	// Store stores the chunk by intercepting a stream.
	Store(resp.AllReadCloser) (resp.AllReadCloser, error)

	// Load loads the data by returning a stream.
	Load() (resp.AllReadCloser, error)

	// LoadAll loads the data by returning the fully loaded data, wait if not fully loaded.
	LoadAll() ([]byte, error)

	// StartPersist instructs the chunk to avoid from being closed before persisted to COS.
	StartPersist(req interface{}, timeout time.Duration, retry PersistRetrier)

	// DonePersist instructs the chunk persistencing has concluded, either success or failure.
	DonePersist()

	// Error returns the error occurred during storing chunk.
	Error() error

	// Close closes the chunk to be removed from the cache.
	Close()

	// Close closes the chunk with specified error.
	CloseWithError(err error)
}