package transfer

import (
	"crypto/rand"
	"encoding/hex"
)

// EngineServiceConfig supplies non-secret engine location and relay settings.
// Transfer credentials travel only through the private worker pipe.
type EngineServiceConfig struct {
	WorkerPath    string
	RelayAddress  string
	RelayPassword string
	EventSink     func(Update)
}

// EngineController extends the bootstrap Service with receive consent and
// sender-code reads required by the desktop host.
type EngineController interface {
	Service
	DecideReceive(transferID, offerID string, accept bool) (bool, error)
	GetReceiveOffer(transferID string) (ReceiveOffer, bool)
	GetTransferSnapshot(transferID string) (SessionSnapshot, bool)
	GetSendCode(transferID string) (string, bool)
	Shutdown()
}

func newConnectionCode() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", ErrWorkerUnavailable
	}
	return hex.EncodeToString(raw[:]), nil
}
