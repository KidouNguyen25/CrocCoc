package transfer

import "time"

const (
	PreparationTimeout         = 2 * time.Minute
	PeerWaitTimeout            = 5 * time.Minute
	OfferDecisionTimeout       = 2 * time.Minute
	PayloadIdleTimeout         = 2 * time.Minute
	WorkerCleanupTimeout       = 10 * time.Second
	WorkerOfferDecisionTimeout = PreparationTimeout + OfferDecisionTimeout
)
