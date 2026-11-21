//go:build windows

package transfer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/windows"
)

// SendSourceLease keeps source files and directories open without write or
// delete sharing until the engine finishes reading the selected tree.
type SendSourceLease struct {
	mu      sync.Mutex
	handles []windows.Handle
	closed  bool
}

// AcquireSendSourceLease locks selected trees and their path ancestors against
// replacement, writes, and reparse-point traversal. Call Close after engine I/O.
func AcquireSendSourceLease(paths []string) (*SendSourceLease, []ManifestEntry, error) {
	return AcquireSendSourceLeaseContext(context.Background(), paths)
}

func AcquireSendSourceLeaseContext(ctx context.Context, paths []string) (*SendSourceLease, []ManifestEntry, error) {
	if ctx == nil {
		return nil, nil, ErrInvalidInput
	}
	if len(paths) == 0 {
		return nil, nil, ErrInvalidInput
	}
	manifest, err := InspectSendSourcesContext(ctx, paths)
	if err != nil {
		return nil, nil, err
	}
	digest, err := SendManifestDigest(manifest)
	if err != nil {
		return nil, nil, err
	}
	lease := &SendSourceLease{}
	for _, selected := range paths {
		if err := ctx.Err(); err != nil {
			_ = lease.Close()
			return nil, nil, err
		}
		if selected == "" || strings.TrimSpace(selected) == "" {
			_ = lease.Close()
			return nil, nil, ErrInvalidInput
		}
		root, err := filepath.Abs(selected)
		if err != nil || !lockSourceAncestors(ctx, lease, root) {
			_ = lease.Close()
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			return nil, nil, ErrSourceUnavailable
		}
		if err := lease.lockTree(ctx, root); err != nil {
			_ = lease.Close()
			return nil, nil, err
		}
	}
	lockedManifest, err := InspectSendSourcesMatchingContext(ctx, paths, digest)
	if err != nil {
		_ = lease.Close()
		return nil, nil, err
	}
	return lease, lockedManifest, nil
}

func lockSourceAncestors(ctx context.Context, lease *SendSourceLease, path string) bool {
	if ctx.Err() != nil {
		return false
	}
	volume := filepath.VolumeName(path)
	if len(volume) != 2 || volume[1] != ':' {
		return false
	}
	root := volume + `\`
	if !lease.openPath(root, true, true) {
		return false
	}
	remainder := strings.TrimLeft(path[len(volume):], `\/`)
	if remainder == "" {
		return true
	}
	parts := strings.Split(remainder, `\`)
	current := root
	for _, part := range parts[:len(parts)-1] {
		if ctx.Err() != nil {
			return false
		}
		current = filepath.Join(current, part)
		if !lease.openPath(current, true, true) {
			return false
		}
	}
	return true
}

func (l *SendSourceLease) lockTree(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	isDir, ok := l.openObject(path, false)
	if !ok {
		return ErrSourceUnavailable
	}
	if !isDir {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return ErrSourceUnavailable
	}
	for _, entry := range entries {
		if err := l.lockTree(ctx, filepath.Join(path, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// openPath opens one directory component. Ancestors allow child writes but
// deny rename; selected tree objects also deny writes.
func (l *SendSourceLease) openPath(path string, directory, ancestor bool) bool {
	share := uint32(windows.FILE_SHARE_READ)
	access := uint32(windows.FILE_READ_ATTRIBUTES)
	if ancestor {
		share |= windows.FILE_SHARE_WRITE
	}
	if directory {
		access |= windows.FILE_LIST_DIRECTORY
	}
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	h, err := windows.CreateFile(ptr, access, share, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return false
	}
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &info) != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(h)
		return false
	}
	l.handles = append(l.handles, h)
	return true
}

func (l *SendSourceLease) openObject(path string, ancestor bool) (bool, bool) {
	share := uint32(windows.FILE_SHARE_READ)
	if ancestor {
		share |= windows.FILE_SHARE_WRITE
	}
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, false
	}
	h, err := windows.CreateFile(ptr, windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY,
		share, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		// Files do not grant FILE_LIST_DIRECTORY. Retry with attribute access only.
		h, err = windows.CreateFile(ptr, windows.FILE_READ_ATTRIBUTES, share, nil,
			windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|
				windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	}
	if err != nil {
		return false, false
	}
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &info) != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(h)
		return false, false
	}
	isDir := info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
	if !isDir {
		fileInfo, statErr := os.Stat(path)
		if statErr != nil || !fileInfo.Mode().IsRegular() {
			_ = windows.CloseHandle(h)
			return false, false
		}
	}
	l.handles = append(l.handles, h)
	return isDir, true
}

// Close releases source locks in reverse acquisition order.
func (l *SendSourceLease) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	var failed bool
	for i := len(l.handles) - 1; i >= 0; i-- {
		failed = windows.CloseHandle(l.handles[i]) != nil || failed
	}
	l.handles = nil
	if failed {
		return ErrSourceUnavailable
	}
	return nil
}
