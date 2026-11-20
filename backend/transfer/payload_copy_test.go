package transfer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

func TestCopyExactPayloadSizeAndEOF(t *testing.T) {
	for _, tc := range []struct {
		name  string
		data  []byte
		size  int64
		valid bool
	}{
		{"empty", nil, 0, true},
		{"exact", bytes.Repeat([]byte("a"), payloadChunkSize*2+3), int64(payloadChunkSize*2 + 3), true},
		{"short", []byte("abc"), 4, false},
		{"long", []byte("abcde"), 4, false},
		{"nonempty zero", []byte("a"), 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := copyExactPayload(context.Background(), &out, bytes.NewReader(tc.data), tc.size)
			if (err == nil) != tc.valid {
				t.Fatalf("got error %v; valid=%v", err, tc.valid)
			}
			if tc.valid && !bytes.Equal(out.Bytes(), tc.data) {
				t.Fatal("payload content altered")
			}
		})
	}
}

type cancelOnRead struct {
	contextCancel context.CancelFunc
	src           *bytes.Reader
	reads         int
}

func (c *cancelOnRead) Read(p []byte) (int, error) {
	c.reads++
	n, err := c.src.Read(p)
	if c.reads == 1 {
		c.contextCancel()
	}
	return n, err
}
func TestCopyExactPayloadCancelsBeforeNextWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &cancelOnRead{contextCancel: cancel, src: bytes.NewReader(bytes.Repeat([]byte("x"), payloadChunkSize*2))}
	var out bytes.Buffer
	err := copyExactPayload(ctx, &out, reader, int64(payloadChunkSize*2))
	if !errors.Is(err, ErrPayloadMismatch) || out.Len() != 0 {
		t.Fatalf("cancelled transfer wrote data: bytes=%d err=%v", out.Len(), err)
	}
}
func TestCopyExactPayloadFailsClosedOnMalformedReader(t *testing.T) {
	var out bytes.Buffer
	if err := copyExactPayload(context.Background(), &out, emptyWithoutEOF{}, 1); !errors.Is(err, ErrPayloadMismatch) {
		t.Fatalf("no-progress stream accepted: %v", err)
	}
	if err := copyExactPayload(nil, &out, bytes.NewReader(nil), 0); !errors.Is(err, ErrPayloadMismatch) {
		t.Fatalf("nil context accepted: %v", err)
	}
	if err := copyExactPayload(context.Background(), io.Discard, bytes.NewReader(nil), MaxManifestBytes+1); !errors.Is(err, ErrPayloadMismatch) {
		t.Fatalf("oversized payload accepted: %v", err)
	}
}

type emptyWithoutEOF struct{}

func (emptyWithoutEOF) Read([]byte) (int, error) { return 0, nil }
