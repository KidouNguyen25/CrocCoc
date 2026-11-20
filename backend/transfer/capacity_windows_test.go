//go:build windows

package transfer

import (
	"errors"
	"testing"
)

func TestNativeDestinationCapacityGate(t *testing.T) {
	lease, err := OpenReceiveDestinationLease(t.TempDir())
	if errors.Is(err, ErrUnsupportedDestination) {
		t.Skip("test directory not on a supported fixed NTFS drive")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err := lease.CheckCapacity(-1); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("negative total escaped quota: %v", err)
	}
	if err := lease.CheckCapacity(0); err != nil && !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("could not read the local quota safely: %v", err)
	}
}
