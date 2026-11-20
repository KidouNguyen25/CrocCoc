//go:build windows

package transfer

import (
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// DestinationLease retains read-only directory handles and their identities.
// Windows filesystems may still permit path renames even without
// FILE_SHARE_DELETE, so VerifyCurrentPath is required for follow-up checks.
// This is a preflight primitive, NOT an authorization to write or publish:
// staging, child handle anchoring, quotas and no-replace publish are separate.
type DestinationLease struct {
	handles []windows.Handle
	id      DirectoryIdentity
	path    string
	once    sync.Once
	closed  atomic.Bool
}

// Identity returns the identity captured from the destination directory handle.
func (l *DestinationLease) Identity() DirectoryIdentity {
	return l.id
}

// VerifyCurrentPath detects a different object currently reachable at the
// originally selected path, including renames/replacements. It does not
// eliminate the race between verification and a later path-based write;
// a native handle-anchored writer is still required before enabling payload.
func (l *DestinationLease) VerifyCurrentPath() error {
	if l == nil || l.closed.Load() {
		return ErrUnsupportedDestination
	}
	now, err := OpenReceiveDestinationLease(l.path)
	if err != nil {
		return ErrUnsupportedDestination
	}
	defer now.Close()
	if now.Identity() != l.id || l.closed.Load() {
		return ErrUnsupportedDestination
	}
	return nil
}

// Close releases all held handles in reverse order. It is safe to call twice.
func (l *DestinationLease) Close() error {
	if l == nil {
		return nil
	}
	var result error
	l.once.Do(func() {
		l.closed.Store(true)
		for i := len(l.handles) - 1; i >= 0; i-- {
			if err := windows.CloseHandle(l.handles[i]); err != nil {
				result = ErrUnsupportedDestination
			}
		}
		l.handles = nil
	})
	return result
}

// OpenReceiveDestinationLease performs read-only destination eligibility
// checks and retains directory handles until Close. It rejects relative,
// UNC, network, non-NTFS, nonexistent and reparse-point paths. No payload
// is requested, staging created or user data changed by this operation.
func OpenReceiveDestinationLease(directory string) (_ *DestinationLease, err error) {
	if directory == "" || !filepath.IsAbs(directory) {
		return nil, ErrUnsupportedDestination
	}
	clean := filepath.Clean(directory)
	volume := filepath.VolumeName(clean)
	if len(volume) != 2 || volume[1] != ':' ||
		!((volume[0] >= 'a' && volume[0] <= 'z') ||
			(volume[0] >= 'A' && volume[0] <= 'Z')) {
		return nil, ErrUnsupportedDestination
	}
	units := 1 // terminating NUL
	for _, r := range clean {
		units += utf16.RuneLen(r)
		if units >= maxWindowsPathUnit {
			return nil, ErrUnsupportedDestination
		}
	}
	root := volume + "\\"
	ptr, err := windows.UTF16PtrFromString(root)
	if err != nil || windows.GetDriveType(ptr) != windows.DRIVE_FIXED {
		return nil, ErrUnsupportedDestination
	}
	var fsName [32]uint16
	if err := windows.GetVolumeInformation(
		ptr, nil, 0, nil, nil, nil, &fsName[0], uint32(len(fsName)),
	); err != nil || !strings.EqualFold(windows.UTF16ToString(fsName[:]), "NTFS") {
		return nil, ErrUnsupportedDestination
	}

	lease := &DestinationLease{path: clean}
	// The caller receives no handle on failure, including failures halfway
	// through traversal. A successfully returned lease owns every handle.
	defer func() {
		if err != nil {
			_ = lease.Close()
		}
	}()
	info, openErr := lease.open(root)
	if openErr != nil {
		return nil, ErrUnsupportedDestination
	}
	lease.id = info
	remainder := strings.TrimLeft(clean[len(volume):], "\\/")
	if remainder == "" {
		return lease, nil
	}
	current := root
	for _, part := range strings.Split(remainder, "\\") {
		if !safeWindowsComponent(part) {
			return nil, ErrUnsupportedDestination
		}
		current = filepath.Join(current, part)
		next, openErr := lease.open(current)
		if openErr != nil || next.VolumeSerial != info.VolumeSerial {
			return nil, ErrUnsupportedDestination
		}
		lease.id = next
	}
	return lease, nil
}

func (l *DestinationLease) open(path string) (DirectoryIdentity, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return DirectoryIdentity{}, ErrUnsupportedDestination
	}
	handle, err := windows.CreateFile(
		ptr, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return DirectoryIdentity{}, ErrUnsupportedDestination
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(handle)
		return DirectoryIdentity{}, ErrUnsupportedDestination
	}
	l.handles = append(l.handles, handle)
	return DirectoryIdentity{
		VolumeSerial: info.VolumeSerialNumber,
		FileIndex:    uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow),
	}, nil
}
