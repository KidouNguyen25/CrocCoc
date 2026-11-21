//go:build windows

package transfer

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func ensurePrivateStageJournalDirectory(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return ErrUnsafeStorage
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return ErrUnsafeStorage
	}
	security, err := privateStageSecurity()
	if err != nil {
		return ErrUnsafeStorage
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ErrUnsafeStorage
	}
	attributes := &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: security,
	}
	err = windows.CreateDirectory(name, attributes)
	runtime.KeepAlive(security)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return ErrUnsafeStorage
	}
	return secureStageJournalDirectory(path)
}

func secureStageJournalDirectory(path string) error {
	security, err := privateStageSecurity()
	if err != nil {
		return ErrUnsafeStorage
	}
	dacl, _, err := security.DACL()
	if err != nil || dacl == nil {
		return ErrUnsafeStorage
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ErrUnsafeStorage
	}
	handle, err := windows.CreateFile(name,
		windows.READ_CONTROL|windows.WRITE_DAC,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return ErrUnsafeStorage
	}
	defer windows.CloseHandle(handle)
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(handle, &info) != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return ErrUnsafeStorage
	}
	if err := windows.SetSecurityInfo(handle, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		return ErrUnsafeStorage
	}
	runtime.KeepAlive(security)
	runtime.KeepAlive(dacl)
	current, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return ErrUnsafeStorage
	}
	control, _, err := current.Control()
	currentDACL, _, daclErr := current.DACL()
	if err != nil || daclErr != nil || currentDACL == nil ||
		control&windows.SE_DACL_PROTECTED == 0 || currentDACL.AceCount != dacl.AceCount {
		return ErrUnsafeStorage
	}
	return nil
}
