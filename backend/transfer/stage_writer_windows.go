//go:build windows

package transfer

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ErrPayloadMismatch = errors.New("TRANSFER_FAILED: nội dung không khớp manifest")

// createChildPayloadFile creates exactly one new ordinary file relative to a
// native directory handle. FILE_CREATE never opens an existing file. It
// validates the returned handle before allowing any payload bytes to be sent.
func createChildPayloadFile(parent windows.Handle, name string, volume uint32) (*os.File, error) {
	if !safeWindowsComponent(name) {
		return nil, ErrUnsafeStorage
	}
	u, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, ErrUnsafeStorage
	}
	attr := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    u,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	var h windows.Handle
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&h,
		windows.FILE_WRITE_DATA|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		&attr, &iosb, nil, windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ,
		windows.FILE_CREATE,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|
			windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	runtime.KeepAlive(u)
	if err != nil {
		return nil, ErrUnsafeStorage
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 ||
		info.VolumeSerialNumber != volume {
		_ = windows.CloseHandle(h)
		return nil, ErrUnsafeStorage
	}
	return os.NewFile(uintptr(h), name), nil
}

// prepareStageDirectories creates only explicitly declared directories,
// anchored to the stage payload handle. It fails closed on an implicit parent,
// a duplicate, a link or an unexpected pre-existing name.
// Callers must have already validated the entire manifest and obtained user
// consent before granting the writer access to the stage.
func (s *ReceiveStage) prepareStageDirectories(entries []ManifestEntry) error {
	if s == nil {
		return ErrUnsafeStorage
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.payload == 0 || s.prepared {
		return ErrUnsafeStorage
	}
	if _, err := ValidateManifest(entries); err != nil {
		return err
	}
	if len(entries) != len(s.allowed) {
		return ErrUnsafeOffer
	}
	for _, entry := range entries {
		entry.Path = strings.ReplaceAll(entry.Path, "\\", "/")
		expected, exists := s.allowed[strings.ToUpper(entry.Path)]
		if !exists || expected != entry {
			return ErrUnsafeOffer
		}
	}
	done := make(map[string]bool)
	for _, entry := range entries {
		if !entry.Directory {
			continue
		}
		path := strings.ReplaceAll(entry.Path, "\\", "/")
		parts := strings.Split(path, "/")
		if len(parts) > 1 && !done[strings.Join(parts[:len(parts)-1], "/")] {
			// Deliberately require an explicit, previously created parent.
			return ErrUnsafeOffer
		}
		parent, err := s.openStageParent(parts[:len(parts)-1])
		if err != nil {
			return err
		}
		h, info, err := createChildDirectory(parent, parts[len(parts)-1])
		if parent != s.payload {
			_ = windows.CloseHandle(parent)
		}
		if err != nil || info.VolumeSerial != s.ownership.Stage.VolumeSerial {
			if h != 0 {
				_ = windows.CloseHandle(h)
			}
			return ErrUnsafeStorage
		}
		if err := windows.CloseHandle(h); err != nil {
			return ErrUnsafeStorage
		}
		done[path] = true
	}
	for _, entry := range entries {
		if entry.Directory {
			continue
		}
		path := strings.ReplaceAll(entry.Path, "\\", "/")
		parts := strings.Split(path, "/")
		if len(parts) > 1 && !done[strings.Join(parts[:len(parts)-1], "/")] {
			return ErrUnsafeOffer
		}
	}
	s.prepared = true
	return nil
}

func (s *ReceiveStage) openStageParent(parts []string) (windows.Handle, error) {
	parent := s.payload
	for _, part := range parts {
		next, _, err := openChildDirectory(parent, part, s.ownership.Stage.VolumeSerial)
		if parent != s.payload {
			_ = windows.CloseHandle(parent)
		}
		if err != nil {
			return 0, err
		}
		parent = next
	}
	return parent, nil
}

// writeStageFile writes an exact finite payload into a freshly created file.
// A short or long stream fails; a partial file is kept inside the owned stage
// and cannot be published by this function. No user destination is touched.
// The caller must validate all manifest entries and ensure acceptance first.
func (s *ReceiveStage) writeStageFile(ctx context.Context, entry ManifestEntry, reader io.Reader) error {
	if s == nil {
		return ErrUnsafeStorage
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.payload == 0 || !s.prepared || entry.Directory || entry.SizeBytes < 0 ||
		entry.SizeBytes > MaxManifestBytes || reader == nil {
		return ErrUnsafeStorage
	}
	entry.Path = strings.ReplaceAll(entry.Path, "\\", "/")
	expected, exists := s.allowed[strings.ToUpper(entry.Path)]
	if !exists || expected != entry {
		return ErrUnsafeOffer
	}
	parts, ok := safeRelativePath(entry.Path)
	if !ok {
		return ErrUnsafeOffer
	}
	parent, err := s.openStageParent(parts[:len(parts)-1])
	if err != nil {
		return err
	}
	defer func() {
		if parent != s.payload {
			_ = windows.CloseHandle(parent)
		}
	}()
	out, err := createChildPayloadFile(parent, parts[len(parts)-1], s.ownership.Stage.VolumeSerial)
	if err != nil {
		return err
	}
	defer out.Close()
	if err := ctx.Err(); err != nil {
		return ErrPayloadMismatch
	}
	// An extra byte is read to distinguish a stream exceeding its declaration
	// from one that matches it exactly. Payload boundaries belong to the
	// framing transport; callers must supply one bounded file reader here.
	written, err := io.CopyN(out, reader, entry.SizeBytes+1)
	if err != nil && err != io.EOF {
		return ErrPayloadMismatch
	}
	if written != entry.SizeBytes {
		return ErrPayloadMismatch
	}
	if err := ctx.Err(); err != nil {
		return ErrPayloadMismatch
	}
	if err := out.Sync(); err != nil {
		return ErrUnsafeStorage
	}
	s.completed[strings.ToUpper(entry.Path)] = true
	return nil
}
