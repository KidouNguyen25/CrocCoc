//go:build windows

package transfer

import (
	"encoding/binary"
	"errors"
	"runtime"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ErrPublishFailed = errors.New("PUBLISH_FAILED: không thể công bố mục nhận an toàn")

// openStagedRoot opens a single file or directory without following any
// reparse point, anchored to an already verified payload directory handle.
// It opens DELETE access because Windows requires it for handle-based rename.
func openStagedRoot(parent windows.Handle, name string, directory bool, volume uint32) (windows.Handle, error) {
	if !safeWindowsComponent(name) {
		return 0, ErrPublishFailed
	}
	u, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, ErrPublishFailed
	}
	attr := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    u,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	flags := uint32(windows.FILE_NON_DIRECTORY_FILE)
	if directory {
		flags = windows.FILE_DIRECTORY_FILE
	}
	var h windows.Handle
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&h,
		windows.DELETE|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		&attr, &iosb, nil, 0,
		windows.FILE_SHARE_READ,
		windows.FILE_OPEN,
		flags|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0, 0)
	runtime.KeepAlive(u)
	if err != nil {
		return 0, ErrPublishFailed
	}
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &info)
	if err != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		(info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) != directory ||
		info.VolumeSerialNumber != volume {
		_ = windows.CloseHandle(h)
		return 0, ErrPublishFailed
	}
	return h, nil
}

// renameNativeNoReplace publishes one already-verified handle to one exact
// top-level name in the native destination directory. ReplaceIfExists is
// strictly false. There is no pre-check/delete/copy fallback, and any error
// leaves a conflicting existing destination item untouched.
//
// The structure layout follows FILE_RENAME_INFO (Windows x64 only):
// BOOLEAN ReplaceIfExists @0, HANDLE RootDirectory @8,
// DWORD FileNameLength @16, UTF-16 WCHAR FileName[] @20.
func renameNativeNoReplace(source, destinationDirectory windows.Handle, newName string) error {
	if !safeWindowsComponent(newName) {
		return ErrPublishFailed
	}
	chars := utf16.Encode([]rune(newName))
	if len(chars) == 0 || len(chars) > 255 {
		return ErrPublishFailed
	}
	// This backend currently targets Windows amd64 exclusively.
	if unsafe.Sizeof(uintptr(0)) != 8 {
		return ErrPublishFailed
	}
	buf := make([]byte, 20+len(chars)*2)
	// buf[0] is ReplaceIfExists and MUST remain 0.
	binary.LittleEndian.PutUint64(buf[8:16], uint64(destinationDirectory))
	binary.LittleEndian.PutUint32(buf[16:20], uint32(len(chars)*2))
	for i, c := range chars {
		binary.LittleEndian.PutUint16(buf[20+i*2:], c)
	}
	var iosb windows.IO_STATUS_BLOCK
	err := windows.NtSetInformationFile(source, &iosb, &buf[0],
		uint32(len(buf)), windows.FileRenameInformation)
	runtime.KeepAlive(buf)
	if err != nil {
		return ErrPublishFailed
	}
	return nil
}
