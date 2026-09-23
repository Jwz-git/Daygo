package app

import (
	"testing"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/domain"
)

// Summary rating bindings are writes on the same database as the card, so the
// tests use a writer backend and a card seeded through the real repository.

func seededCardID(t *testing.T, backend *Backend) int64 {
	t.Helper()
	seedTimelineDay(t, backend, []domain.CardShell{
		{Start: "10:00 AM", End: "10:30 AM", Category: "Coding", Title: "c1", Summary: "s"},
	})
	dto, err := backend.GetTimelineDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetTimelineDay: %v", err)
	}
	if len(dto.Cards) != 1 {
		t.Fatalf("seeded %d cards, want 1", len(dto.Cards))
	}
	return dto.Cards[0].ID
}

func TestCardRatingRoundTrip(t *testing.T) {
	backend, _ := writerBackendWithStore(t, t.TempDir())
	cardID := seededCardID(t, backend)

	// An unrated card reads back as the empty string, not as an error: the
	// inspector shows no active thumb rather than a failure state.
	rating, err := backend.GetCardRating(cardID)
	if err != nil {
		t.Fatalf("GetCardRating: %v", err)
	}
	if rating != "" {
		t.Fatalf("rating = %q before any vote, want empty", rating)
	}

	if err := backend.SaveCardRating(cardID, "up"); err != nil {
		t.Fatalf("SaveCardRating: %v", err)
	}
	rating, err = backend.GetCardRating(cardID)
	if err != nil {
		t.Fatal(err)
	}
	if rating != "up" {
		t.Fatalf("rating = %q, want up", rating)
	}

	// Flipping the vote overwrites rather than stacking.
	if err := backend.SaveCardRating(cardID, "down"); err != nil {
		t.Fatalf("re-rate: %v", err)
	}
	rating, err = backend.GetCardRating(cardID)
	if err != nil {
		t.Fatal(err)
	}
	if rating != "down" {
		t.Fatalf("rating = %q, want down", rating)
	}

	if err := backend.ClearCardRating(cardID); err != nil {
		t.Fatalf("ClearCardRating: %v", err)
	}
	rating, err = backend.GetCardRating(cardID)
	if err != nil {
		t.Fatal(err)
	}
	if rating != "" {
		t.Fatalf("rating = %q after clear, want empty", rating)
	}
}

func TestCardRatingRejectsBadInput(t *testing.T) {
	backend, _ := writerBackendWithStore(t, t.TempDir())
	cardID := seededCardID(t, backend)

	// The stored set is closed: a value the frontend never sends is refused
	// instead of being written and read back as an unknown thumb.
	assertAppCode(t, backend.SaveCardRating(cardID, "sideways"), apperr.InvalidArgument)
	assertAppCode(t, backend.SaveCardRating(0, "up"), apperr.InvalidArgument)
	assertAppCode(t, backend.SaveCardRating(-1, "up"), apperr.InvalidArgument)
	assertAppCode(t, backend.ClearCardRating(0), apperr.InvalidArgument)

	rating, err := backend.GetCardRating(cardID)
	if err != nil {
		t.Fatal(err)
	}
	if rating != "" {
		t.Fatalf("rating = %q after rejected writes, want empty", rating)
	}

	// A card that does not exist cannot be rated.
	assertAppCode(t, backend.SaveCardRating(999999, "up"), apperr.NotFound)
}

func TestCardRatingReadOnlyInstanceRefusesWrites(t *testing.T) {
	dir := t.TempDir()
	writerBackendWithStore(t, dir) // holds the write lock

	// A second instance on the same directory is read-only at the connection
	// layer, so the thumbs must refuse there instead of pretending to save.
	readerStore := openTestStore(t, dir, true)
	backend := newBackend(fixedClock{}, nil, readerStore, false, false)
	backend.setEventEmitter(&recordingEmitter{})

	assertAppCode(t, backend.SaveCardRating(1, "up"), apperr.NotCaptureOwner)
	assertAppCode(t, backend.ClearCardRating(1), apperr.NotCaptureOwner)

	// Reads stay available: a read-only instance still shows the stored thumb.
	rating, err := backend.GetCardRating(1)
	if err != nil {
		t.Fatalf("GetCardRating on a read-only instance: %v", err)
	}
	if rating != "" {
		t.Fatalf("rating = %q on an unrated card, want empty", rating)
	}
}
