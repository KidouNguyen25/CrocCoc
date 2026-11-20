package transfer

import (
	"errors"
	"testing"
)

func TestReceiveCapacityBudgetBoundaries(t *testing.T) {
	reserve := MinReceiveFreeSpaceReserve
	cases := []struct {
		name      string
		available uint64
		payload   int64
		wantError bool
	}{
		{"empty without reserve", 0, 0, true},
		{"empty below reserve", reserve - 1, 0, true},
		{"empty with exact reserve", reserve, 0, false},
		{"one byte insufficient", reserve, 1, true},
		{"one byte enough", reserve + 1, 1, false},
		{"max payload insufficient", uint64(MaxManifestBytes) + reserve - 1, MaxManifestBytes, true},
		{"max payload exact", uint64(MaxManifestBytes) + reserve, MaxManifestBytes, false},
		{"negative payload", ^uint64(0), -1, true},
		{"payload exceeds manifest maximum", ^uint64(0), MaxManifestBytes + 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckReceiveBudget(tc.available, tc.payload)
			if tc.wantError && !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("want resource limit, got %v", err)
			}
			if !tc.wantError && err != nil {
				t.Fatalf("valid budget rejected: %v", err)
			}
		})
	}
}
