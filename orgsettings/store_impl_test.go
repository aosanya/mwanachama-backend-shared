package orgsettings_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings"
)

func TestStoreGetUnknownSlugReturnsErrNotFound(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Get(context.Background(), "unknown"); !errors.Is(err, orgsettings.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown slug, got %v", err)
	}
}

func TestStorePutThenGetRoundTrips(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	first := orgsettings.Settings{
		Slug: "mwanachama",
		Attributes: map[string]any{
			"display_name":  "Mwanachama",
			"primary_color": "#000000",
			"accent_color":  "#ffffff",
		},
	}
	if _, err := s.Put(ctx, first); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := s.Get(ctx, "mwanachama")
	if err != nil {
		t.Fatalf("get after put: %v", err)
	}
	if got.DisplayName() != "Mwanachama" || got.PrimaryColor() != "#000000" {
		t.Fatalf("wrong settings: %+v", got)
	}

	// Put upserts: a second Put with the same slug overwrites, including
	// the very first insert (the ON CONFLICT path — regression coverage for
	// the "GORM Save silently no-ops on a fresh slug" bug this store
	// deliberately avoids by using an explicit clause.OnConflict).
	updated := orgsettings.Settings{
		Slug: "mwanachama",
		Attributes: map[string]any{
			"display_name":  "Mwanachama Movement",
			"primary_color": "#123456",
			"accent_color":  "#abcdef",
			"logo_url":      "https://example.test/logo.png",
		},
	}
	if _, err := s.Put(ctx, updated); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err = s.Get(ctx, "mwanachama")
	if err != nil {
		t.Fatalf("get after upsert: %v", err)
	}
	if got.DisplayName() != "Mwanachama Movement" || got.LogoURL() == "" {
		t.Fatalf("expected upsert to replace fields, got %+v", got)
	}
}

// TestStorePutRejectsInvalidAttribute mirrors
// mwanachama-backend-actor's TestCreateActorRejectsInvalidAttribute-shaped
// coverage: a Range violation is refused before any row is written.
func TestStorePutRejectsInvalidAttribute(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.Put(ctx, orgsettings.Settings{
		Slug:       "badregion",
		Attributes: map[string]any{"default_dialling_region": 254},
	})
	if !errors.Is(err, orgsettings.ErrInvalidSettings) {
		t.Fatalf("expected ErrInvalidSettings for a non-text region, got %v", err)
	}
	if _, getErr := s.Get(ctx, "badregion"); !errors.Is(getErr, orgsettings.ErrNotFound) {
		t.Fatalf("a rejected Put must not have written a row, got %v", getErr)
	}
}
