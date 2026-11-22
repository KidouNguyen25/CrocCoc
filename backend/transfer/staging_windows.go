//go:build windows

package transfer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const stagePrefix = ".croccoc-stage-"

// StageOwnership is the durable local recovery record for one stage.
// Recovery verifies the marker and native identities before deleting data.
type StageOwnership struct {
	StageName       string            `json:"stageName"`
	MarkerNonce     string            `json:"markerNonce"`
	DestinationPath string            `json:"destinationPath"`
	Destination     DirectoryIdentity `json:"destination"`
	Stage           DirectoryIdentity `json:"stage"`
	Status          string            `json:"status"`
	CreatedAt       time.Time         `json:"createdAt"`
}

// ReceiveStage keeps handles for a uniquely created same-volume directory.
// Only storage code with an explicit handle should create payload children.
// Close releases handles; it intentionally DOES NOT remove any directory.
type ReceiveStage struct {
	StageName        string
	stage            windows.Handle
	payload          windows.Handle
	journalLock      windows.Handle
	journalLockPath  string
	journalPath      string
	ownership        StageOwnership
	mu               sync.Mutex
	allowed          map[string]ManifestEntry
	completed        map[string]DirectoryIdentity
	stageDirectories map[string]DirectoryIdentity
	prepared         bool
	destinationPath  string
	payloadID        DirectoryIdentity
	manifestDigest   [32]byte
}

func (s *ReceiveStage) Ownership() StageOwnership {
	return s.ownership
}

func (s *ReceiveStage) JournalPath() string {
	return s.journalPath
}

// PayloadPath returns the private directory that the reviewed engine worker
// may use for payload writes. The caller must retain the stage ownership lease
// and audit this path before publication.
func (s *ReceiveStage) PayloadPath() string {
	if s == nil || s.StageName == "" || s.destinationPath == "" {
		return ""
	}
	return filepath.Join(s.destinationPath, s.StageName, "payload")
}

// Close releases native handles without guessing whether it is safe to
// remove stage data. Recovery/cleanup requires a separate verified operation.
func (s *ReceiveStage) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.closeHandlesLocked(); err != nil {
		return ErrUnsafeStorage
	}
	return s.closeJournalLockLocked()
}

func (s *ReceiveStage) closeHandlesLocked() error {
	var failed bool
	if s.payload != 0 {
		if windows.CloseHandle(s.payload) == nil {
			s.payload = 0
		} else {
			failed = true
		}
	}
	if s.stage != 0 {
		if windows.CloseHandle(s.stage) == nil {
			s.stage = 0
		} else {
			failed = true
		}
	}
	if failed {
		return ErrUnsafeStorage
	}
	return nil
}

func (s *ReceiveStage) closeJournalLockLocked() error {
	if s.journalLock == 0 {
		return nil
	}
	handle := s.journalLock
	s.journalLock = 0
	if windows.CloseHandle(handle) != nil {
		return ErrUnsafeStorage
	}
	return nil
}

// PrepareReceiveStage writes no payload. It first acquires an exclusive,
// random directory via native RootDirectory handles, creates an empty payload
// directory and a random marker, then flushes a user-local ownership journal.
// A stage without a durable ownership journal is NEVER returned to a worker.
//
// On errors between native stage creation and journal sync, leave the stage
// behind for manual inspection; do not blindly delete by a path or prefix.
func PrepareReceiveStage(lease *DestinationLease, entries []ManifestEntry) (*ReceiveStage, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		return nil, ErrUnsafeStorage
	}
	journalDir := filepath.Join(cacheDir, "CrocCoc", "staging")
	return prepareReceiveStageAt(lease, entries, journalDir)
}

func prepareReceiveStageAt(lease *DestinationLease, entries []ManifestEntry, journalDir string) (_ *ReceiveStage, err error) {
	if lease == nil || lease.VerifyCurrentPath() != nil {
		return nil, ErrUnsafeStorage
	}
	if err := CheckReceivePathBudget(lease.path, entries); err != nil {
		return nil, err
	}
	total, err := ValidateManifest(entries)
	if err != nil {
		return nil, err
	}
	if err := lease.CheckCapacity(total); err != nil {
		return nil, err
	}
	if err := lease.CheckReceiveRootVacancy(entries); err != nil {
		return nil, err
	}
	root, err := openWritableDestination(lease)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(root)

	allowed := make(map[string]ManifestEntry, len(entries))
	for _, entry := range entries {
		entry.Path = strings.ReplaceAll(entry.Path, "\\", "/")
		allowed[strings.ToUpper(entry.Path)] = entry
	}
	journalCommitted := false
	journalCreated := false
	stage := &ReceiveStage{allowed: allowed, completed: make(map[string]DirectoryIdentity), destinationPath: lease.path, manifestDigest: fingerprintManifest(entries)}
	defer func() {
		if err != nil {
			_ = stage.Close()
			if !journalCommitted && stage.journalLockPath != "" {
				_ = os.Remove(stage.journalLockPath)
			}
			// A partial journal would fail recovery and block every launch.
			if !journalCommitted && journalCreated {
				_ = os.Remove(stage.journalPath)
			}
		}
	}()
	var stageID, marker [16]byte
	if _, err := rand.Read(stageID[:]); err != nil {
		return nil, ErrUnsafeStorage
	}
	if _, err := rand.Read(marker[:]); err != nil {
		return nil, ErrUnsafeStorage
	}
	stage.StageName = stagePrefix + hex.EncodeToString(stageID[:])
	var stageIdentity DirectoryIdentity
	security, err := privateStageSecurity()
	if err != nil {
		return nil, err
	}
	stage.stage, stageIdentity, err = createChildDirectoryWithSecurity(root, stage.StageName, security)
	if err != nil {
		return nil, err
	}
	if stageIdentity.VolumeSerial != lease.id.VolumeSerial {
		return nil, ErrUnsafeStorage
	}
	var payloadIdentity DirectoryIdentity
	stage.payload, payloadIdentity, err = createChildDirectory(stage.stage, "payload")
	if err != nil || payloadIdentity.VolumeSerial != stageIdentity.VolumeSerial {
		return nil, ErrUnsafeStorage
	}
	stage.payloadID = payloadIdentity
	nonce := hex.EncodeToString(marker[:])
	if err := createStageMarker(stage.stage, nonce); err != nil {
		return nil, err
	}
	stage.ownership = StageOwnership{
		StageName:       stage.StageName,
		MarkerNonce:     nonce,
		DestinationPath: lease.path,
		Destination:     lease.id,
		Stage:           stageIdentity,
		Status:          "prepared",
		CreatedAt:       time.Now().UTC(),
	}
	if err := ensurePrivateStageJournalDirectory(journalDir); err != nil {
		return nil, ErrUnsafeStorage
	}
	journalID := hex.EncodeToString(stageID[:])
	stage.journalPath = filepath.Join(journalDir, journalID+".json")
	stage.journalLockPath = filepath.Join(journalDir, journalID+".lock")
	stage.journalLock, err = createStageJournalLock(stage.journalLockPath)
	if err != nil {
		return nil, ErrUnsafeStorage
	}
	journal, err := os.OpenFile(stage.journalPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, ErrUnsafeStorage
	}
	journalCreated = true
	data, err := json.Marshal(stage.ownership)
	if err != nil {
		_ = journal.Close()
		return nil, ErrUnsafeStorage
	}
	if _, err = journal.Write(append(data, '\n')); err != nil {
		_ = journal.Close()
		return nil, ErrUnsafeStorage
	}
	if err = journal.Sync(); err != nil {
		_ = journal.Close()
		return nil, ErrUnsafeStorage
	}
	if err = journal.Close(); err != nil {
		return nil, ErrUnsafeStorage
	}
	journalCommitted = true
	// Native payload writing still requires a separately validated writer.
	return stage, nil
}

func createStageJournalLock(path string) (windows.Handle, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, ErrUnsafeStorage
	}
	handle, err := windows.CreateFile(ptr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil, windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return 0, ErrUnsafeStorage
	}
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(handle, &info) != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(handle)
		return 0, ErrUnsafeStorage
	}
	return handle, nil
}

func createStageMarker(parent windows.Handle, nonce string) error {
	name, err := windows.NewNTUnicodeString(".croccoc-owner")
	if err != nil {
		return ErrUnsafeStorage
	}
	attr := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    name,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	var h windows.Handle
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&h, windows.FILE_WRITE_DATA|windows.SYNCHRONIZE,
		&attr, &iosb, nil, windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ,
		windows.FILE_CREATE,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|
			windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	runtime.KeepAlive(name)
	if err != nil {
		return ErrUnsafeStorage
	}
	defer windows.CloseHandle(h)
	var written uint32
	if err := windows.WriteFile(h, []byte(nonce), &written, nil); err != nil ||
		written != uint32(len(nonce)) {
		return ErrUnsafeStorage
	}
	if err := windows.FlushFileBuffers(h); err != nil {
		return ErrUnsafeStorage
	}
	return nil
}

// StageErrorIsRecoverable reports that no payload-write capability was
// created when storage failed. The error itself never includes paths.
func StageErrorIsRecoverable(err error) bool {
	return errors.Is(err, ErrUnsafeStorage)
}
