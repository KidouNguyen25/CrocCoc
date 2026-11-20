//go:build !windows

package transfer

// CheckReceiveDestinationEligibility fails closed on unsupported platforms.
// The MVP does not permit Windows-specific transfer operations elsewhere.
func CheckReceiveDestinationEligibility(string) error {
	return ErrUnsupportedDestination
}
