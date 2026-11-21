//go:build windows

package transfer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ErrStageCleanup = errors.New("CLEANUP_FAILED: không thể dọn vùng nhận đã xác minh")

func cleanupPublishedReceiveStage(lease *DestinationLease, stage *ReceiveStage) error {
	if lease == nil || stage == nil {
		return stageCleanupError("missing ownership")
	}
	stage.mu.Lock()
	defer stage.mu.Unlock()
	if stage.ownership.Status != "published" || stage.journalPath == "" ||
		lease.Identity() != stage.ownership.Destination || lease.VerifyCurrentPath() != nil {
		return stageCleanupError("ownership mismatch")
	}
	if err := stage.closeHandlesLocked(); err != nil {
		return stageCleanupError("close staging handles")
	}
	if err := verifyStageJournal(stage); err != nil {
		return stageCleanupError("journal validation")
	}
	destination, err := openWritableDestination(lease)
	if err != nil {
		return stageCleanupError("open destination")
	}
	defer windows.CloseHandle(destination)

	stageHandle, stageIdentity, err := openChildDirectory(destination, stage.StageName, stage.ownership.Destination.VolumeSerial)
	if err != nil || stageIdentity != stage.ownership.Stage {
		if stageHandle != 0 {
			_ = windows.CloseHandle(stageHandle)
		}
		return stageCleanupError("open owned stage")
	}
	payloadPath := filepath.Join(stage.destinationPath, stage.StageName, "payload")
	items, err := os.ReadDir(payloadPath)
	if err != nil || len(items) != 0 {
		_ = windows.CloseHandle(stageHandle)
		return stageCleanupError("payload directory is not empty")
	}
	payloadHandle, err := openDeletionDirectory(stageHandle, "payload", stage.payloadID)
	if err != nil {
		_ = windows.CloseHandle(stageHandle)
		return stageCleanupError("open payload directory")
	}

	payloadDeleteErr := markHandleForDeletion(payloadHandle)
	payloadCloseErr := windows.CloseHandle(payloadHandle)
	if payloadDeleteErr != nil || payloadCloseErr != nil {
		_ = windows.CloseHandle(stageHandle)
		return stageCleanupError("delete payload directory")
	}
	marker, err := openDeletionFile(stageHandle, ".croccoc-owner", stage.ownership.Stage.VolumeSerial)
	if err != nil {
		_ = windows.CloseHandle(stageHandle)
		return stageCleanupError("open owner marker")
	}
	markerBytes := make([]byte, len(stage.ownership.MarkerNonce)+1)
	var read uint32
	readErr := windows.ReadFile(marker, markerBytes, &read, nil)
	markerErr := error(nil)
	if readErr != nil ||
		int(read) != len(stage.ownership.MarkerNonce) ||
		!bytes.Equal(markerBytes[:read], []byte(stage.ownership.MarkerNonce)) {
		markerErr = ErrStageCleanup
	} else {
		markerErr = markHandleForDeletion(marker)
	}
	markerCloseErr := windows.CloseHandle(marker)
	if markerErr != nil || markerCloseErr != nil {
		_ = windows.CloseHandle(stageHandle)
		return stageCleanupError("verify owner marker")
	}
	stageCloseErr := windows.CloseHandle(stageHandle)
	if stageCloseErr != nil {
		return stageCleanupError("close stage directory")
	}
	items, err = os.ReadDir(filepath.Join(stage.destinationPath, stage.StageName))
	if err != nil {
		return stageCleanupError("stage directory inspection failed")
	}
	if len(items) != 0 {
		return stageCleanupError("stage directory is not empty")
	}
	stageHandle, err = openDeletionDirectory(destination, stage.StageName, stage.ownership.Stage)
	if err != nil {
		return stageCleanupError("reopen owned stage for deletion")
	}
	stageDeleteErr := markHandleForDeletion(stageHandle)
	stageCloseErr = windows.CloseHandle(stageHandle)
	if stageDeleteErr != nil || stageCloseErr != nil {
		return stageCleanupError("delete stage directory")
	}
	if err := os.Remove(stage.journalPath); err != nil {
		return stageCleanupError("remove ownership journal")
	}
	return nil
}

func stageCleanupError(reason string) error {
	return fmt.Errorf("%w: %s", ErrStageCleanup, reason)
}

func verifyStageJournal(stage *ReceiveStage) error {
	info, err := os.Lstat(stage.journalPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return ErrStageCleanup
	}
	data, err := os.ReadFile(stage.journalPath)
	if err != nil {
		return ErrStageCleanup
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var ownership StageOwnership
	if decoder.Decode(&ownership) != nil || decoder.Decode(new(json.RawMessage)) != io.EOF || ownership != stage.ownership {
		return ErrStageCleanup
	}
	return nil
}

func openDeletionDirectory(parent windows.Handle, name string, expected DirectoryIdentity) (windows.Handle, error) {
	if !safeWindowsComponent(name) || expected.FileIndex == 0 {
		return 0, ErrStageCleanup
	}
	u, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, ErrStageCleanup
	}
	attr := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    u,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	var handle windows.Handle
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle,
		windows.DELETE|windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY|windows.SYNCHRONIZE,
		&attr, &iosb, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		windows.FILE_OPEN,
		windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0, 0)
	runtime.KeepAlive(u)
	if err != nil {
		return 0, ErrStageCleanup
	}
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(handle, &info) != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		stagedIdentityFromHandle(handle) != expected {
		_ = windows.CloseHandle(handle)
		return 0, ErrStageCleanup
	}
	return handle, nil
}

func openDeletionFile(parent windows.Handle, name string, volume uint32) (windows.Handle, error) {
	if !safeWindowsComponent(name) {
		return 0, ErrStageCleanup
	}
	u, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, ErrStageCleanup
	}
	attr := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    u,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	var handle windows.Handle
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle,
		windows.DELETE|windows.FILE_READ_DATA|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		&attr, &iosb, nil, 0,
		windows.FILE_SHARE_READ,
		windows.FILE_OPEN,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0, 0)
	runtime.KeepAlive(u)
	if err != nil {
		return 0, ErrStageCleanup
	}
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(handle, &info) != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.VolumeSerialNumber != volume {
		_ = windows.CloseHandle(handle)
		return 0, ErrStageCleanup
	}
	return handle, nil
}

func markHandleForDeletion(handle windows.Handle) error {
	deleteFile := byte(1)
	var iosb windows.IO_STATUS_BLOCK
	return windows.NtSetInformationFile(handle, &iosb, &deleteFile, uint32(unsafe.Sizeof(deleteFile)), windows.FileDispositionInformation)
}
