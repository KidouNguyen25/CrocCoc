// Package transfer owns the Go transfer service and its transport-neutral contract.
package transfer

import (
	"errors"
	"strings"
)

const (
	UpdateEvent       = "transfer:update"
	PinnedCrocVersion = "11.6.4"
)

var ErrNotIntegrated = errors.New("Croc chưa được tích hợp trong bản khởi tạo.")

var ErrTransferFailed = errors.New("TRANSFER_FAILED: truyền tệp thất bại")

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

type TransferError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Update struct {
	TransferID       string         `json:"transferID"`
	Revision         uint64         `json:"revision,omitempty"`
	State            State          `json:"state"`
	CurrentFile      string         `json:"currentFile,omitempty"`
	BytesTransferred *int64         `json:"bytesTransferred,omitempty"`
	BytesTotal       *int64         `json:"bytesTotal,omitempty"`
	BytesPerSecond   *int64         `json:"bytesPerSecond,omitempty"`
	Progress         *float64       `json:"progress,omitempty"`
	Message          string         `json:"message,omitempty"`
	ConnectionCode   string         `json:"connectionCode,omitempty"`
	ReceiveOffer     *ReceiveOffer  `json:"receiveOffer,omitempty"`
	Error            *TransferError `json:"error,omitempty"`
	PartialOutput    bool           `json:"partialOutput,omitempty"`
	CleanupPending   bool           `json:"cleanupPending,omitempty"`
}

func transferError(err error) *TransferError {
	if err == nil {
		return nil
	}
	message := err.Error()
	code := "TRANSFER_FAILED"
	if separator := strings.Index(message, ": "); separator > 0 {
		code = message[:separator]
		message = message[separator+2:]
	}
	return &TransferError{Code: code, Message: message}
}
