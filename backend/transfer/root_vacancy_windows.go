//go:build windows

package transfer

import (
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CheckReceiveRootVacancy ensures none of the incoming root names currently
// resolves to an existing NTFS directory entry or short-name alias. Unlike
// os.ReadDir-based name comparisons, the lookup is anchored to a verified
// destination directory handle and handled case-insensitively by NTFS.
//
// This is an early check, not a reservation: no-replace publish is mandatory
// and the check must be repeated immediately before starting payload receive.
func (l *DestinationLease) CheckReceiveRootVacancy(entries []ManifestEntry) error {
	if l == nil || l.VerifyCurrentPath() != nil {
		return ErrUnsupportedDestination
	}
	roots, err := explicitRootItems(entries)
	if err != nil {
		return err
	}
	parent, err := openWritableDestination(l)
	if err != nil {
		return ErrUnsupportedDestination
	}
	defer windows.CloseHandle(parent)
	for _, entry := range roots {
		unicodeName, err := windows.NewNTUnicodeString(strings.ReplaceAll(entry.Path, "\\", "/"))
		if err != nil {
			return ErrUnsafeOffer
		}
		attrs := windows.OBJECT_ATTRIBUTES{
			Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
			RootDirectory: parent,
			ObjectName:    unicodeName,
			Attributes:    windows.OBJ_CASE_INSENSITIVE,
		}
		var handle windows.Handle
		var iosb windows.IO_STATUS_BLOCK
		err = windows.NtCreateFile(
			&handle, windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
			&attrs, &iosb, nil, 0,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
			windows.FILE_OPEN,
			windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
			0, 0)
		runtime.KeepAlive(unicodeName)
		if err == nil {
			_ = windows.CloseHandle(handle)
			return ErrDestinationConflict
		}
		if err != windows.STATUS_OBJECT_NAME_NOT_FOUND {
			return ErrUnsupportedDestination
		}
	}
	if err := l.VerifyCurrentPath(); err != nil {
		return err
	}
	return nil
}
