// Package transfer owns the Go transfer service and its transport-neutral contract.
package transfer

import "errors"

const (
	UpdateEvent       = "transfer:update"
	PinnedCrocVersion = "11.6.4"
)

var ErrNotIntegrated = errors.New("Croc chưa được tích hợp trong bản khởi tạo.")

type State string

const (
	Idle               State = "IDLE"
	Preparing          State = "PREPARING"
	WaitingForPeer     State = "WAITING_FOR_PEER"
	AwaitingAcceptance State = "AWAITING_ACCEPTANCE"
	Transferring       State = "TRANSFERRING"
	Finalizing         State = "FINALIZING"
	Cancelling         State = "CANCELLING"
	Completed          State = "COMPLETED"
	Failed             State = "FAILED"
	Cancelled          State = "CANCELLED"
)

type CrocInfo struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Path      string `json:"path,omitempty"`
	Message   string `json:"message,omitempty"`
}

type Update struct {
	TransferID       string   `json:"transferID"`
	State            State    `json:"state"`
	CurrentFile      string   `json:"currentFile,omitempty"`
	BytesTransferred *int64   `json:"bytesTransferred,omitempty"`
	BytesTotal       *int64   `json:"bytesTotal,omitempty"`
	BytesPerSecond   *int64   `json:"bytesPerSecond,omitempty"`
	Progress         *float64 `json:"progress,omitempty"`
	Message          string   `json:"message,omitempty"`
	ConnectionCode   string   `json:"connectionCode,omitempty"`
}
