//go:build !windows

package transfer

// DestinationLease cannot be opened outside supported Windows platforms.
type DestinationLease struct{}

// OpenReceiveDestinationLease fails closed outside Windows.
func OpenReceiveDestinationLease(string) (*DestinationLease, error) {
	return nil, ErrUnsupportedDestination
}

func (*DestinationLease) Close() error { return nil }

func (*DestinationLease) Identity() DirectoryIdentity { return DirectoryIdentity{} }

func (*DestinationLease) VerifyCurrentPath() error { return ErrUnsupportedDestination }

// CheckReceiveDestinationEligibility fails closed on unsupported platforms.
func CheckReceiveDestinationEligibility(string) error {
	return ErrUnsupportedDestination
}
