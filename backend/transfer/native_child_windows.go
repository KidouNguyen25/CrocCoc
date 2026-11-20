//go:build windows

package transfer

import (
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ntFileAddFile         = 0x0002
	ntFileAddSubdirectory = 0x0004
)

// ErrUnsafeStorage never includes a machine path or NTSTATUS in UI output.
var ErrUnsafeStorage = errors.New("UNSUPPORTED_DESTINATION: không thể tạo vùng lưu trữ an toàn")

// createChildDirectory uses the given native directory object as RootDirectory.
// The name must be exactly one safe Windows component. FILE_CREATE is atomic:
// it fails if that name already exists; it never opens or replaces an entry.
func createChildDirectory(parent windows.Handle, name string) (windows.Handle, DirectoryIdentity, error) {
	if !safeWindowsComponent(name) {
		return 0, DirectoryIdentity{}, ErrUnsafeStorage
	}
	unicodeName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, DirectoryIdentity{}, ErrUnsafeStorage
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    unicodeName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	var handle windows.Handle
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle,
		windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY|
			ntFileAddFile|ntFileAddSubdirectory|
			windows.SYNCHRONIZE,
		&attributes, &iosb, nil, windows.FILE_ATTRIBUTE_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		windows.FILE_CREATE,
		windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|
			windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	runtime.KeepAlive(unicodeName)
	if err != nil {
		return 0, DirectoryIdentity{}, ErrUnsafeStorage
	}
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(handle, &info); err != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(handle)
		return 0, DirectoryIdentity{}, ErrUnsafeStorage
	}
	return handle, DirectoryIdentity{
		VolumeSerial: info.VolumeSerialNumber,
		FileIndex:    uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow),
	}, nil
}

// openChildDirectory opens a direct child without following a reparse point,
// and confirms that the object is a plain directory on the parent's volume.
func openChildDirectory(parent windows.Handle, name string, parentVolume uint32) (windows.Handle, DirectoryIdentity, error) {
	if !safeWindowsComponent(name) {
		return 0, DirectoryIdentity{}, ErrUnsafeStorage
	}
	unicodeName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, DirectoryIdentity{}, ErrUnsafeStorage
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    unicodeName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	var handle windows.Handle
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle,
		windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY|windows.SYNCHRONIZE,
		&attributes, &iosb, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		windows.FILE_OPEN,
		windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|
			windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	runtime.KeepAlive(unicodeName)
	if err != nil {
		return 0, DirectoryIdentity{}, ErrUnsafeStorage
	}
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(handle, &info); err != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.VolumeSerialNumber != parentVolume {
		_ = windows.CloseHandle(handle)
		return 0, DirectoryIdentity{}, ErrUnsafeStorage
	}
	return handle, DirectoryIdentity{
		VolumeSerial: info.VolumeSerialNumber,
		FileIndex:    uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow),
	}, nil
}
