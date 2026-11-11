package create

import (
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
)

func TestHashInputUsesStableAPIRepresentation(t *testing.T) {
	t.Parallel()

	input := domaintrip.NewTrip{
		UserID:     uuid.MustParse("5cb72c04-7650-45c9-a79b-bcdba0631e0c"),
		DriverID:   uuid.MustParse("8860b315-ec86-42eb-a17c-7c163d721ff5"),
		StartPoint: domaintrip.Coordinates{Latitude: 59.9398, Longitude: 30.3146},
		EndPoint:   domaintrip.Coordinates{Latitude: 59.929, Longitude: 30.3626},
		Price:      1450,
	}
	result, err := hashInput(input, true)
	if err != nil {
		t.Fatalf("hash input: %v", err)
	}
	expected := sha256.Sum256([]byte(
		`{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c",` +
			`"driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5",` +
			`"start_point":{"latitude":59.9398,"longitude":30.3146},` +
			`"end_point":{"latitude":59.929,"longitude":30.3626},"price":1450}`,
	))
	if string(result) != string(expected[:]) {
		t.Fatalf("idempotency hash changed: got %x, want %x", result, expected)
	}
}
