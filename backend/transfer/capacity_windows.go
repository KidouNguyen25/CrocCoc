//go:build windows

package transfer

import "golang.org/x/sys/windows"

// CheckCapacity reads the filesystem's free bytes available to this account
// (not just global free space). Native quota/accounting and actual write
// enforcement are required later: a positive result is NOT a reservation.
func (l *DestinationLease) CheckCapacity(payloadBytes int64) error {
	if err := l.VerifyCurrentPath(); err != nil {
		return ErrUnsupportedDestination
	}
	path, err := windows.UTF16PtrFromString(l.path)
	if err != nil {
		return ErrUnsupportedDestination
	}
	var available uint64
	if err := windows.GetDiskFreeSpaceEx(path, &available, nil, nil); err != nil {
		return ErrUnsupportedDestination
	}
	if err := l.VerifyCurrentPath(); err != nil {
		return ErrUnsupportedDestination
	}
	return CheckReceiveBudget(available, payloadBytes)
}
