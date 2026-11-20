package transfer

// MinReceiveFreeSpaceReserve is the extra room required for a receiver's
// staging metadata and bounded incidental filesystem work (256 MiB).
const MinReceiveFreeSpaceReserve = uint64(256) << 20

// CheckReceiveBudget evaluates the amount actually available to the account,
// including applicable user quotas. This is only one preflight gate: it must
// be rechecked before writing and enforced while receiving payload bytes.
func CheckReceiveBudget(available uint64, payloadBytes int64) error {
	if payloadBytes < 0 || payloadBytes > MaxManifestBytes {
		return ErrResourceLimit
	}
	needed := uint64(payloadBytes) + MinReceiveFreeSpaceReserve
	if available < needed {
		return ErrResourceLimit
	}
	return nil
}
