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

func newConnectionCode() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", ErrWorkerUnavailable
	}
	return hex.EncodeToString(raw[:]), nil
}
