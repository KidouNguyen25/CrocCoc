package transfer

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

type fixtureWriterAt []byte

func (f fixtureWriterAt) WriteAt(p []byte, off int64) (int, error) {
	return copy(f[off:], p), nil
}

func TestPayloadChunkWriterRequiresEveryChunkInAnyOrder(t *testing.T) {
	dst := make(fixtureWriterAt, crocPayloadChunkSize+3)
	w, err := newPayloadChunkWriter(t.Context(), dst, int64(len(dst)))
	if err != nil {
		t.Fatal(err)
	}
	first := bytes.Repeat([]byte{'a'}, crocPayloadChunkSize)
	for _, chunk := range []struct {
		offset int64
		data   []byte
	}{
		{crocPayloadChunkSize, []byte("end")},
		{0, first},
	} {
		if n, err := w.WriteAt(chunk.data, chunk.offset); n != len(chunk.data) || err != nil {
			t.Fatalf("chunk rejected: n=%d err=%v", n, err)
		}
	}
	if err := w.seal(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dst[:crocPayloadChunkSize], first) || string(dst[crocPayloadChunkSize:]) != "end" {
		t.Fatal("staged bytes do not match sorted payload")
	}
	if _, err := w.WriteAt(first, 0); !errors.Is(err, ErrPayloadMismatch) {
		t.Fatal("sealed writer allowed a write")
	}
}

func TestPayloadChunkWriterFailsClosedOnInvalidOrIncompleteWrite(t *testing.T) {
	for _, tc := range []struct {
		name   string
		offset int64
		data   []byte
		replay bool
	}{
		{"negative offset", -1, []byte("x"), false},
		{"unaligned", 1, []byte("x"), false},
		{"past end", crocPayloadChunkSize, []byte("x"), false},
		{"too long", 0, []byte("toolong"), false},
		{"too short", 0, []byte("x"), false},
		{"duplicate", 0, []byte("two"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dst := fixtureWriterAt(make([]byte, 3))
			w, err := newPayloadChunkWriter(t.Context(), dst, int64(len(dst)))
			if err != nil {
				t.Fatal(err)
			}
			if tc.replay {
				if _, err := w.WriteAt([]byte("one"), 0); err != nil {
					t.Fatal(err)
				}
			}
			if n, err := w.WriteAt(tc.data, tc.offset); n != 0 || !errors.Is(err, ErrPayloadMismatch) {
				t.Fatalf("invalid chunk accepted: n=%d err=%v", n, err)
			}
			if err := w.seal(); !errors.Is(err, ErrPayloadMismatch) {
				t.Fatalf("invalidated file accepted: %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	dst := fixtureWriterAt(make([]byte, 3))
	w, err := newPayloadChunkWriter(ctx, dst, int64(len(dst)))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := w.WriteAt([]byte("one"), 0); !errors.Is(err, ErrPayloadMismatch) {
		t.Fatal("cancelled writer accepted data")
	}
	if err := w.seal(); !errors.Is(err, ErrPayloadMismatch) {
		t.Fatal("cancelled file sealed as complete")
	}

	w, err = newPayloadChunkWriter(t.Context(), dst, int64(len(dst)))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.seal(); !errors.Is(err, ErrPayloadMismatch) {
		t.Fatal("missing data was accepted")
	}
	w, err = newPayloadChunkWriter(t.Context(), dst, 0)
	if err != nil || w.seal() != nil {
		t.Fatal("empty file was not complete")
	}
}
