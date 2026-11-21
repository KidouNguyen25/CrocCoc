//go:build windows && !croc_no_tailcat

package transfer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrWorkerPinMismatch = errors.New("ENGINE_UNAVAILABLE: engine không khớp bản đã ghim")

// pinWorkerExecutable checks the exact worker bytes and keeps its file tree
// locked against replacement for the lifetime of the service.
func pinWorkerExecutable(path, expectedSHA256 string) (*SendSourceLease, error) {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return nil, ErrWorkerPinMismatch
	}
	expected, err := hex.DecodeString(expectedSHA256)
	if err != nil || len(expected) != sha256.Size {
		return nil, ErrWorkerPinMismatch
	}
	lease, _, err := AcquireSendSourceLease([]string{path})
	if err != nil {
		return nil, ErrWorkerPinMismatch
	}
	file, err := os.Open(path)
	if err != nil {
		_ = lease.Close()
		return nil, ErrWorkerPinMismatch
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || !bytes.Equal(hash.Sum(nil), expected) {
		_ = lease.Close()
		return nil, ErrWorkerPinMismatch
	}
	return lease, nil
}
