//go:build !windows

package transfer

// PreflightReceiveDestination refuses native storage operations elsewhere.
func PreflightReceiveDestination(string, []ManifestEntry) (*DestinationLease, error) {
	return nil, ErrUnsupportedDestination
}
