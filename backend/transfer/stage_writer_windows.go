//go:build windows

package transfer

import (
	"context"
	"io"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// createChildPayloadFile creates exactly one new ordinary file relative to a
// native directory handle. FILE_CREATE never opens an existing file. It
// validates the returned handle before allowing any payload bytes to be sent.
func createChildPayloadFile(parent windows.Handle, name string, volume uint32) (*os.File, DirectoryIdentity, error) {
	if !safeWindowsComponent(name) {
		return nil, DirectoryIdentity{}, ErrUnsafeStorage
	}
	u, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, DirectoryIdentity{}, ErrUnsafeStorage
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
		return nil, DirectoryIdentity{}, ErrUnsafeStorage
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 ||
		info.VolumeSerialNumber != volume {
		_ = windows.CloseHandle(h)
		return nil, DirectoryIdentity{}, ErrUnsafeStorage
	}
	return os.NewFile(uintptr(h), name), stagedObjectIdentity(info), nil
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
	s.stageDirectories = make(map[string]DirectoryIdentity)
	for _, entry := range entries {
		if !entry.Directory {
			continue
		}
		path := strings.ReplaceAll(entry.Path, "\\", "/")
		parts := strings.Split(path, "/")
		if len(parts) > 1 && s.stageDirectories[strings.Join(parts[:len(parts)-1], "/")] == (DirectoryIdentity{}) {
			// Deliberately require an explicit, previously created parent.
			return ErrUnsafeOffer
		}
		parent, closeParents, err := s.openStageParent(parts[:len(parts)-1])
		if err != nil {
			return err
		}
		h, info, err := createChildDirectory(parent, parts[len(parts)-1])
		closeParents()
		if err != nil || info.VolumeSerial != s.ownership.Stage.VolumeSerial {
			if h != 0 {
				_ = windows.CloseHandle(h)
			}
			return ErrUnsafeStorage
		}
		if err := windows.CloseHandle(h); err != nil {
			return ErrUnsafeStorage
		}
		s.stageDirectories[path] = info
	}
	for _, entry := range entries {
		if entry.Directory {
			continue
		}
		path := strings.ReplaceAll(entry.Path, "\\", "/")
		parts := strings.Split(path, "/")
		if len(parts) > 1 && s.stageDirectories[strings.Join(parts[:len(parts)-1], "/")] == (DirectoryIdentity{}) {
			return ErrUnsafeOffer
		}
	}
	s.prepared = true
	return nil
}

// Keep every verified directory in the chain open without delete sharing
// until the write finishes. Descendant operations never re-resolve an
// unverified ancestor by name.
func (s *ReceiveStage) openStageParent(parts []string) (windows.Handle, func(), error) {
	parent := s.payload
	opened := make([]windows.Handle, 0, len(parts))
	closeParents := func() {
		for i := len(opened) - 1; i >= 0; i-- {
			_ = windows.CloseHandle(opened[i])
		}
	}
	var path string
	for _, part := range parts {
		if path != "" {
			path += "/"
		}
		path += part
		expected, found := s.stageDirectories[path]
		if !found {
			closeParents()
			return 0, nil, ErrUnsafeStorage
		}
		next, actual, err := openChildDirectory(parent, part, s.ownership.Stage.VolumeSerial)
		if err != nil {
			closeParents()
			return 0, nil, err
		}
		opened = append(opened, next)
		if actual != expected {
			closeParents()
			return 0, nil, ErrUnsafeStorage
		}
		parent = next
	}
	return parent, closeParents, nil
}

// writeStageFile writes an exact finite payload into a freshly created file.
// A short or long stream fails; a partial file is kept inside the owned stage
// and cannot be published by this function. No user destination is touched.
// The caller must validate all manifest entries and ensure acceptance first.
func (s *ReceiveStage) writeStageFile(ctx context.Context, entry ManifestEntry, reader io.Reader) error {
	if reader == nil {
		return ErrUnsafeStorage
	}
	return s.writeStagePayload(ctx, entry, func(out *os.File) error {
		return copyExactPayload(ctx, out, reader, entry.SizeBytes)
	})
}

// writeStageChunks holds the verified staging chain for one fresh file while
// an engine supplies the pinned relay path's aligned chunks. The io.WriterAt
// is revoked when consume returns. It is not an engine receive permit.
func (s *ReceiveStage) writeStageChunks(ctx context.Context, entry ManifestEntry, consume func(io.WriterAt) error) error {
	if consume == nil {
		return ErrUnsafeStorage
	}
	return s.writeStagePayload(ctx, entry, func(out *os.File) error {
		writer, err := newPayloadChunkWriter(ctx, out, entry.SizeBytes)
		if err != nil {
			return err
		}
		consumeErr := consume(writer)
		finishErr := writer.seal()
		if consumeErr != nil {
			return ErrPayloadMismatch
		}
		return finishErr
	})
}

func (s *ReceiveStage) writeStagePayload(ctx context.Context, entry ManifestEntry, write func(*os.File) error) error {
	if s == nil {
		return ErrUnsafeStorage
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.payload == 0 || !s.prepared || entry.Directory || entry.SizeBytes < 0 ||
		entry.SizeBytes > MaxManifestBytes {
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
	if ctx == nil || ctx.Err() != nil {
		return ErrPayloadMismatch
	}
	parent, closeParents, err := s.openStageParent(parts[:len(parts)-1])
	if err != nil {
		return err
	}
	defer closeParents()
	out, id, err := createChildPayloadFile(parent, parts[len(parts)-1], s.ownership.Stage.VolumeSerial)
	if err != nil {
		return err
	}
	defer out.Close()
	if err := write(out); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return ErrUnsafeStorage
	}
	if err := out.Close(); err != nil {
		return ErrUnsafeStorage
	}
	if ctx.Err() != nil {
		return ErrPayloadMismatch
	}
	s.completed[strings.ToUpper(entry.Path)] = id
	return nil
}
