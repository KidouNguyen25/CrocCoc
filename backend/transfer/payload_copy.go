package transfer

import (
	"context"
	"errors"
	"io"
)

var ErrPayloadMismatch = errors.New("TRANSFER_FAILED: nội dung không khớp manifest")

const payloadChunkSize = 64 * 1024

// copyExactPayload checks cancellation between bounded read/write chunks and
// demands EOF exactly at the declared byte length. The reader must represent
// ONE framed file stream and be closed by the engine on cancellation to unblock
// any currently blocked Read; context alone cannot interrupt an arbitrary
// io.Reader. No progress estimates are inferred from caller-supplied metadata.
func copyExactPayload(ctx context.Context, dst io.Writer, src io.Reader, length int64) error {
	if ctx == nil || dst == nil || src == nil || length < 0 ||
		length > MaxManifestBytes {
		return ErrPayloadMismatch
	}
	buf := make([]byte, payloadChunkSize)
	remaining := length
	for remaining > 0 {
		if ctx.Err() != nil {
			return ErrPayloadMismatch
		}
		next := payloadChunkSize
		if remaining < int64(next) {
			next = int(remaining)
		}
		n, err := src.Read(buf[:next])
		if n < 0 || n > next || ctx.Err() != nil {
			return ErrPayloadMismatch
		}
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			if writeErr != nil || written != n {
				return ErrPayloadMismatch
			}
			remaining -= int64(n)
		}
		if err != nil && err != io.EOF {
			return ErrPayloadMismatch
		}
		if (err == io.EOF && remaining != 0) || (n == 0 && err == nil) {
			return ErrPayloadMismatch
		}
	}
	if ctx.Err() != nil {
		return ErrPayloadMismatch
	}
	var extra [1]byte
	n, err := src.Read(extra[:])
	if ctx.Err() != nil || n != 0 || err != io.EOF {
		return ErrPayloadMismatch
	}
	return nil
}
