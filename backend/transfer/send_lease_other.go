//go:build !windows

package transfer

// SendSourceLease cannot lock native source trees outside Windows.
type SendSourceLease struct{}

func AcquireSendSourceLease([]string) (*SendSourceLease, []ManifestEntry, error) {
	return nil, nil, ErrSourceUnavailable
}

func (*SendSourceLease) Close() error { return nil }
