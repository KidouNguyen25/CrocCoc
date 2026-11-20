//go:build windows

package transfer

import (
	"golang.org/x/sys/windows"
)

// openWritableDestination grants only local parent-directory capabilities,
// and checks the returned handle identity against the retained read-only
// lease before creating any child. A later operation uses this handle,
// never a path-based destination resolved independently.
func openWritableDestination(lease *DestinationLease) (windows.Handle, error) {
	if lease == nil || lease.VerifyCurrentPath() != nil {
		return 0, ErrUnsafeStorage
	}
	ptr, err := windows.UTF16PtrFromString(lease.path)
	if err != nil {
		return 0, ErrUnsafeStorage
	}
	h, err := windows.CreateFile(ptr,
		ntFileAddSubdirectory|ntFileAddFile|
			windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return 0, ErrUnsafeStorage
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.VolumeSerialNumber != lease.Identity().VolumeSerial ||
		(uint64(info.FileIndexHigh)<<32|uint64(info.FileIndexLow)) != lease.Identity().FileIndex {
		_ = windows.CloseHandle(h)
		return 0, ErrUnsafeStorage
	}
	return h, nil
}
