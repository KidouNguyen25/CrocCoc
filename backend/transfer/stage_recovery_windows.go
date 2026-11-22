//go:build windows && !croc_no_tailcat

package transfer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

var ErrRecoveryPending = errors.New("CLEANUP_FAILED: vùng nhận cần phục hồi trước phiên mới")

func recoverOwnedReceiveStages() error {
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		return ErrRecoveryPending
	}
	return recoverOwnedReceiveStagesAt(filepath.Join(cacheDir, "CrocCoc", "staging"))
}

func recoverOwnedReceiveStagesAt(journalDir string) error {
	if _, err := os.Lstat(journalDir); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil || secureStageJournalDirectory(journalDir) != nil {
		return ErrRecoveryPending
	}
	entries, err := os.ReadDir(journalDir)
	if err != nil {
		return ErrRecoveryPending
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if err := recoverOwnedReceiveStage(filepath.Join(journalDir, entry.Name())); err != nil {
			return ErrRecoveryPending
		}
	}
	return nil
}

func recoverOwnedReceiveStage(journalPath string) error {
	lockPath := strings.TrimSuffix(journalPath, filepath.Ext(journalPath)) + ".lock"
	lock, err := openStageRecoveryLock(lockPath)
	if err != nil {
		return ErrRecoveryPending
	}
	defer func() {
		if lock != 0 {
			_ = windows.CloseHandle(lock)
		}
	}()

	ownership, err := readStageOwnership(journalPath)
	if err != nil || !validStageOwnership(ownership, journalPath) {
		return ErrRecoveryPending
	}
	lease, err := OpenReceiveDestinationLease(ownership.DestinationPath)
	if err != nil {
		// A destination that no longer exists took its stage with it; keeping
		// the journal would block every future launch.
		if _, statErr := os.Lstat(ownership.DestinationPath); errors.Is(statErr, os.ErrNotExist) {
			return finishStageRecovery(journalPath, lockPath, &lock)
		}
		return ErrRecoveryPending
	}
	if lease.Identity() != ownership.Destination {
		_ = lease.Close()
		return ErrRecoveryPending
	}
	stagePath := filepath.Join(ownership.DestinationPath, ownership.StageName)
	if _, err := os.Lstat(stagePath); errors.Is(err, os.ErrNotExist) {
		_ = lease.Close()
		return finishStageRecovery(journalPath, lockPath, &lock)
	} else if err != nil {
		_ = lease.Close()
		return ErrRecoveryPending
	}
	root := lease.handles[len(lease.handles)-1]
	stage, identity, err := openChildDirectory(root, ownership.StageName, ownership.Destination.VolumeSerial)
	if err != nil || identity != ownership.Stage {
		if stage != 0 {
			_ = windows.CloseHandle(stage)
		}
		_ = lease.Close()
		return ErrRecoveryPending
	}
	marker, err := openDeletionFile(stage, ".croccoc-owner", ownership.Stage.VolumeSerial)
	if err != nil {
		_ = windows.CloseHandle(stage)
		_ = lease.Close()
		return ErrRecoveryPending
	}
	markerBytes := make([]byte, len(ownership.MarkerNonce)+1)
	var count uint32
	readErr := windows.ReadFile(marker, markerBytes, &count, nil)
	markerValid := readErr == nil && int(count) == len(ownership.MarkerNonce) &&
		bytes.Equal(markerBytes[:count], []byte(ownership.MarkerNonce))
	markerCloseErr := windows.CloseHandle(marker)
	if !markerValid || markerCloseErr != nil {
		_ = windows.CloseHandle(stage)
		_ = lease.Close()
		return ErrRecoveryPending
	}
	// The verified stage handle stays open without delete sharing, so the
	// directory cannot be renamed or replaced while its children are removed.
	children, readErr := os.ReadDir(stagePath)
	removeErr := readErr
	for i := 0; removeErr == nil && i < len(children); i++ {
		removeErr = os.RemoveAll(filepath.Join(stagePath, children[i].Name()))
	}
	stageCloseErr := windows.CloseHandle(stage)
	if removeErr != nil || stageCloseErr != nil {
		_ = lease.Close()
		return ErrRecoveryPending
	}
	// Remove only deletes an empty directory, so a replacement is never emptied.
	if err := os.Remove(stagePath); err != nil {
		_ = lease.Close()
		return ErrRecoveryPending
	}
	if err := lease.Close(); err != nil {
		return ErrRecoveryPending
	}
	return finishStageRecovery(journalPath, lockPath, &lock)
}

func finishStageRecovery(journalPath, lockPath string, lock *windows.Handle) error {
	if err := os.Remove(journalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return ErrRecoveryPending
	}
	if *lock != 0 {
		err := windows.CloseHandle(*lock)
		*lock = 0
		if err != nil {
			return ErrRecoveryPending
		}
	}
	_ = os.Remove(lockPath)
	return nil
}

func openStageRecoveryLock(path string) (windows.Handle, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, ErrRecoveryPending
	}
	lock, err := windows.CreateFile(ptr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil, windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return 0, ErrRecoveryPending
	}
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(lock, &info) != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(lock)
		return 0, ErrRecoveryPending
	}
	return lock, nil
}

func readStageOwnership(path string) (StageOwnership, error) {
	var ownership StageOwnership
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ownership, ErrRecoveryPending
	}
	handle, err := windows.CreateFile(ptr, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return ownership, ErrRecoveryPending
	}
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(handle, &info) != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(handle)
		return ownership, ErrRecoveryPending
	}
	file := os.NewFile(uintptr(handle), path)
	metadata, err := file.Stat()
	if err != nil || metadata.Size() > 8192 {
		_ = file.Close()
		return ownership, ErrRecoveryPending
	}
	data, err := io.ReadAll(io.LimitReader(file, 8193))
	closeErr := file.Close()
	if err != nil || closeErr != nil || len(data) > 8192 {
		return ownership, ErrRecoveryPending
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&ownership) != nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
		return StageOwnership{}, ErrRecoveryPending
	}
	return ownership, nil
}

func validStageOwnership(ownership StageOwnership, journalPath string) bool {
	if !filepath.IsAbs(ownership.DestinationPath) ||
		!safeWindowsComponent(ownership.StageName) ||
		!strings.HasPrefix(ownership.StageName, stagePrefix) ||
		ownership.Status != "prepared" && ownership.Status != "publishing" &&
			ownership.Status != "partial" && ownership.Status != "published" ||
		ownership.Destination.FileIndex == 0 || ownership.Stage.FileIndex == 0 ||
		ownership.Destination.VolumeSerial != ownership.Stage.VolumeSerial ||
		len(ownership.MarkerNonce) != 32 {
		return false
	}
	stageID := strings.TrimPrefix(ownership.StageName, stagePrefix)
	if decoded, err := hex.DecodeString(stageID); err != nil || len(decoded) != 16 {
		return false
	}
	if decoded, err := hex.DecodeString(ownership.MarkerNonce); err != nil || len(decoded) != 16 {
		return false
	}
	return strings.TrimSuffix(filepath.Base(journalPath), filepath.Ext(journalPath)) == stageID
}
