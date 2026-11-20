//go:build !windows

package transfer

// CheckCapacity fails closed on platforms without the Windows NTFS gate.
func (*DestinationLease) CheckCapacity(int64) error {
	return ErrUnsupportedDestination
}
