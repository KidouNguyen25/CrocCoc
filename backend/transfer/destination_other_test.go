//go:build !windows

package transfer

import (
	"errors"
	"testing"
)

func TestDestinationEligibilityFailsClosedOutsideWindows(t *testing.T) {
	for _, path := range []string{"", "/tmp", "C:\\Users\\Public\\Downloads"} {
		if err := CheckReceiveDestinationEligibility(path); !errors.Is(err, ErrUnsupportedDestination) {
			t.Fatalf("unexpected non-Windows destination accepted: %q: %v", path, err)
		}
	}
}
