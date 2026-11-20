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

// StageOwnership is the durable, secret-free recovery anchor for one stage.
// The journal by itself never authorizes deletion: recovery must verify
// both the marker nonce and native directory identities before touching data.
type StageOwnership struct {
	StageName   string            `json:"stageName"`
	MarkerNonce string            `json:"markerNonce"`
	Destination DirectoryIdentity `json:"destination"`
	Stage       DirectoryIdentity `json:"stage"`
	Status      string            `json:"status"`
	CreatedAt   time.Time         `json:"createdAt"`
}

// ReceiveStage keeps handles for a uniquely created same-volume directory.
// Only storage code with an explicit handle should create payload children.
// Close releases handles; it intentionally DOES NOT remove any directory.
type ReceiveStage struct {
	StageName       string
	stage           windows.Handle
	payload         windows.Handle
	journalPath     string
	ownership       StageOwnership
	mu              sync.Mutex
	allowed         map[string]ManifestEntry
	completed       map[string]bool
	prepared        bool
	destinationPath string
	payloadID       DirectoryIdentity
	manifestDigest  [32]byte
}

func (s *ReceiveStage) Ownership() StageOwnership {
	return s.ownership
}

func (s *ReceiveStage) JournalPath() string {
	return s.journalPath
}

// Close releases native handles without guessing whether it is safe to
// remove stage data. Recovery/cleanup requires a separate verified operation.
func (s *ReceiveStage) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var failed bool
	if s.payload != 0 {
		failed = windows.CloseHandle(s.payload) != nil
		s.payload = 0
	}
	if s.stage != 0 {
		failed = windows.CloseHandle(s.stage) != nil || failed
		s.stage = 0
	}
	if failed {
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
	stage := &ReceiveStage{allowed: allowed, completed: make(map[string]bool), destinationPath: lease.path, manifestDigest: fingerprintManifest(entries)}
	defer func() {
		if err != nil {
			_ = stage.Close()
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
	stage.stage, stageIdentity, err = createChildDirectory(root, stage.StageName)
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
		StageName:   stage.StageName,
		MarkerNonce: nonce,
		Destination: lease.id,
		Stage:       stageIdentity,
		Status:      "prepared",
		CreatedAt:   time.Now().UTC(),
	}
	if err := os.MkdirAll(journalDir, 0700); err != nil {
		return nil, ErrUnsafeStorage
	}
	stage.journalPath = filepath.Join(journalDir, hex.EncodeToString(stageID[:])+".json")
	journal, err := os.OpenFile(stage.journalPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, ErrUnsafeStorage
	}
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
	// Native payload writing still requires a separately validated writer.
	return stage, nil
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
