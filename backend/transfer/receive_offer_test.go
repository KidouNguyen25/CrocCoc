package transfer

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestReceiveOfferContainsOnlySanitizedMetadata(t *testing.T) {
	now := time.Date(2026, 11, 20, 2, 0, 0, 0, time.UTC)
	raw := []ManifestEntry{
		{Path: "Tài liệu", Directory: true},
		{Path: "Tài liệu\\báo cáo.txt", SizeBytes: 7},
		{Path: "empty.txt", SizeBytes: 0},
	}
	offer, err := PrepareReceiveOffer(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(offer.OfferID) != 32 || offer.FileCount != 2 || offer.TotalBytes != "7" {
		t.Fatalf("invalid offer: %#v", offer)
	}
	if !offer.ExpiresAt.Equal(now.Add(OfferDecisionTimeout)) ||
		offer.Expired(now.Add(OfferDecisionTimeout-time.Nanosecond)) ||
		!offer.Expired(now.Add(OfferDecisionTimeout)) {
		t.Fatal("offer deadline is not enforced")
	}
	if offer.Items[0].Type != "directory" || offer.Items[0].SizeBytes != "" ||
		offer.Items[1].Path != "Tài liệu/báo cáo.txt" ||
		offer.Items[1].SizeBytes != "7" || offer.Items[2].SizeBytes != "0" {
		t.Fatalf("unsafe item metadata: %#v", offer.Items)
	}
	raw[1].Path = "mutated.txt"
	if offer.Items[1].Path != "Tài liệu/báo cáo.txt" {
		t.Fatal("offer retained mutable peer metadata")
	}
	data, err := json.Marshal(offer)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "mutated") || strings.Contains(string(data), "\\\\") {
		t.Fatalf("wire leaked raw metadata: %s", data)
	}
}

func TestReceiveOfferCopiesAndIdentifiers(t *testing.T) {
	first, err := PrepareReceiveOffer([]ManifestEntry{{Path: "a.txt"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	second, err := PrepareReceiveOffer([]ManifestEntry{{Path: "a.txt"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if first.OfferID == second.OfferID {
		t.Fatal("reused offer identifier")
	}
	clone := first.Clone()
	clone.Items[0].Path = "changed"
	if first.Items[0].Path != "a.txt" {
		t.Fatal("clone shares mutable backing slice")
	}
}

func TestReceiveOfferRejectsUnsafeManifestBeforeIssuingID(t *testing.T) {
	for _, invalid := range [][]ManifestEntry{
		nil,
		{{Path: "CON"}},
		{{Path: "a.txt", SizeBytes: MaxManifestBytes + 1}},
	} {
		offer, err := PrepareReceiveOffer(invalid, time.Now())
		if err == nil || offer.OfferID != "" || len(offer.Items) != 0 {
			t.Fatalf("unsafe offer accepted: %#v, %v", offer, err)
		}
		if len(invalid) == 1 && invalid[0].SizeBytes > MaxManifestBytes &&
			!errors.Is(err, ErrResourceLimit) {
			t.Fatalf("quota error lost: %v", err)
		}
	}
}
