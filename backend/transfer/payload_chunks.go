package transfer

import (
	"context"
	"io"
	"sync"
)

// The pinned Croc v11.6.4 relay data path uses models.TCP_BUFFER_SIZE/2.
const crocPayloadChunkSize = 32 * 1024

// payloadChunkWriter is for fresh files only. It accepts one write per Croc
// chunk without allowing holes, overlaps, or an extension of the staged file.
// The bounded bitset does not grow with duplicate or malformed requests.
type payloadChunkWriter struct {
	mu        sync.Mutex
	ctx       context.Context
	dst       io.WriterAt
	size      int64
	remaining int64
	seen      []uint64
	failed    bool
	sealed    bool
}

func newPayloadChunkWriter(ctx context.Context, dst io.WriterAt, size int64) (*payloadChunkWriter, error) {
	if ctx == nil || ctx.Err() != nil || dst == nil || size < 0 || size > MaxManifestBytes {
		return nil, ErrPayloadMismatch
	}
	chunks := (size + crocPayloadChunkSize - 1) / crocPayloadChunkSize
	return &payloadChunkWriter{
		ctx: ctx, dst: dst, size: size, remaining: chunks,
		seen: make([]uint64, (chunks+63)/64),
	}, nil
}

func (w *payloadChunkWriter) WriteAt(data []byte, offset int64) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.sealed || w.failed {
		return 0, ErrPayloadMismatch
	}
	if w.ctx.Err() != nil || offset < 0 || offset >= w.size || offset%crocPayloadChunkSize != 0 ||
		int64(len(data)) != min(int64(crocPayloadChunkSize), w.size-offset) {
		w.failed = true
		return 0, ErrPayloadMismatch
	}
	index := offset / crocPayloadChunkSize
	word, bit := index/64, uint64(1)<<uint(index%64)
	if w.seen[word]&bit != 0 {
		w.failed = true
		return 0, ErrPayloadMismatch
	}
	n, err := w.dst.WriteAt(data, offset)
	if err != nil || n != len(data) || w.ctx.Err() != nil {
		w.failed = true
		return n, ErrPayloadMismatch
	}
	w.seen[word] |= bit
	w.remaining--
	return n, nil
}

// seal permanently revokes the writer, including after cancellation or failure.
func (w *payloadChunkWriter) seal() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.sealed {
		return ErrPayloadMismatch
	}
	w.sealed = true
	if w.failed || w.remaining != 0 || w.ctx.Err() != nil {
		return ErrPayloadMismatch
	}
	return nil
}
