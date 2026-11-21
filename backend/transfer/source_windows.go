//go:build windows

package transfer

import "golang.org/x/sys/windows"

// sourcePathIsReparsePoint inspects the selected object itself. OPEN_REPARSE_POINT
// prevents a junction or symlink from being followed during this check.
func sourcePathIsReparsePoint(path string) (bool, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	h, err := windows.CreateFile(
		ptr, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(h)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return false, err
	}
	return info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}
