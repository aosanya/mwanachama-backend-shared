package orgsettings_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings"
	"github.com/aosanya/mwanachama-backend-shared/orgsettings/models"
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

	first := models.Settings{
		Slug:         "mwanachama",
		DisplayName:  "Mwanachama",
		PrimaryColor: "#000000",
		AccentColor:  "#ffffff",
	}
	if _, err := s.Put(ctx, first); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := s.Get(ctx, "mwanachama")
	if err != nil {
		t.Fatalf("get after put: %v", err)
	}
	if got.DisplayName != first.DisplayName || got.PrimaryColor != first.PrimaryColor {
		t.Fatalf("wrong settings: %+v", got)
	}

	// Put upserts: a second Put with the same slug overwrites, including
	// the very first insert (the ON CONFLICT path — regression coverage for
	// the "GORM Save silently no-ops on a fresh slug" bug this store
	// deliberately avoids by using an explicit clause.OnConflict).
	updated := models.Settings{
		Slug:         "mwanachama",
		DisplayName:  "Mwanachama Movement",
		PrimaryColor: "#123456",
		AccentColor:  "#abcdef",
		LogoURL:      "https://example.test/logo.png",
	}
	if _, err := s.Put(ctx, updated); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err = s.Get(ctx, "mwanachama")
	if err != nil {
		t.Fatalf("get after upsert: %v", err)
	}
	if got.DisplayName != "Mwanachama Movement" || got.LogoURL == "" {
		t.Fatalf("expected upsert to replace fields, got %+v", got)
	}
}
