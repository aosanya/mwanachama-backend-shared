package identity_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/identity"
	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func TestKeyOfIsTheFullSHA256HexOfTheRaw(t *testing.T) {
	const raw = "agency/agy1f2e3d4c/R-3"
	const want = "9e4c428222a646cc03fb8515392443988679cb73f027af9b764762b377fdb180"
	if got := identity.KeyOf(raw); got != want {
		t.Fatalf("KeyOf(%q) = %q, want %q", raw, got, want)
	}
	if !identity.KeyPattern.MatchString(identity.KeyOf(raw)) {
		t.Fatalf("a key must match %s", identity.KeyPattern)
	}
	if identity.KeyOf("git/auth/store.go") == identity.KeyOf("git/auth/store.go ") {
		t.Fatalf("two raws differing by a byte must not share a key")
	}
	r, err := identity.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Key() != want {
		t.Fatalf("Raw.Key() = %q, want KeyOf(String()) = %q", r.Key(), want)
	}
}

func TestParseSplitsOnTheFirstTwoSlashesOnly(t *testing.T) {
	r, err := identity.Parse("git/auth/internal/store.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Module != "git" || r.Instance != "auth" || r.Identity != "internal/store.go" {
		t.Fatalf("parsed %+v", r)
	}
	if r.String() != "git/auth/internal/store.go" {
		t.Fatalf("round trip: %q", r.String())
	}
	if _, err := identity.Parse("tm/inst1/a b c"); err != nil {
		t.Fatalf("an identity may carry inner whitespace: %v", err)
	}
	if _, err := identity.Parse("tm/inst1/ünïcödé"); err != nil {
		t.Fatalf("an identity may be any UTF-8: %v", err)
	}
}

func TestNewBuildsTheSameRawParseReads(t *testing.T) {
	r, err := identity.New("agency", "agy1f2e3d4c", "R-3")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	parsed, err := identity.Parse(r.String())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed != r {
		t.Fatalf("New gave %+v, Parse read back %+v", r, parsed)
	}
	for _, bad := range [][3]string{
		{"Agency", "inst", "R-3"},
		{"agency", "in_st", "R-3"},
		{"agency", "inst", ""},
		{"agency", "inst", " R-3"},
		{"", "inst", "R-3"},
	} {
		if _, err := identity.New(bad[0], bad[1], bad[2]); !errors.Is(err, identity.ErrInvalidRaw) {
			t.Errorf("New(%q, %q, %q): want ErrInvalidRaw, got %v", bad[0], bad[1], bad[2], err)
		}
	}
}

func TestParseRefusesWhatTheAlphabetForbids(t *testing.T) {
	for _, bad := range []string{
		"",
		"agency",
		"agency/inst",
		"agency/inst/",
		"Agency/inst/R-3",
		"agency/In_st/R-3",
		"agency/1inst/R-3",
		"/inst/R-3",
		"agency//R-3",
		"agency/inst/ R-3",
		"agency/inst/R-3\n",
		"agency/inst/R\x00-3",
		"agency/inst/\xff",
	} {
		if _, err := identity.Parse(bad); !errors.Is(err, identity.ErrInvalidRaw) {
			t.Errorf("Parse(%q): want ErrInvalidRaw, got %v", bad, err)
		}
	}
}

func TestASegmentIsHeldToTheSpecAlphabet(t *testing.T) {
	if identity.SegmentPattern.String() != spec.SegmentPattern.String() {
		t.Fatalf("identity.SegmentPattern %s drifted from spec.SegmentPattern %s", identity.SegmentPattern, spec.SegmentPattern)
	}
}

type fakeOwner struct {
	module   string
	instance string
	rows     map[string]string
}

func (o fakeOwner) StableIdentity(_ context.Context, object, id string) (string, error) {
	code, ok := o.rows[object+"/"+id]
	if !ok {
		return "", fmt.Errorf("%s %s: %w", object, id, identity.ErrNotFound)
	}
	r, err := identity.New(o.module, o.instance, code)
	if err != nil {
		return "", err
	}
	return r.String(), nil
}

func (o fakeOwner) Resolve(_ context.Context, raw string) (string, string, error) {
	r, err := identity.Parse(raw)
	if err != nil {
		return "", "", err
	}
	if r.Module != o.module || r.Instance != o.instance {
		return "", "", fmt.Errorf("%s: %w", raw, identity.ErrNotOwned)
	}
	for key, code := range o.rows {
		if code == r.Identity {
			object, id, _ := splitKey(key)
			return object, id, nil
		}
	}
	return "", "", fmt.Errorf("%s: %w", raw, identity.ErrNotFound)
}

func splitKey(key string) (string, string, bool) {
	for i := range key {
		if key[i] == '/' {
			return key[:i], key[i+1:], true
		}
	}
	return "", "", false
}

func TestAnOwnerAnswersBothDirectionsAndNamesAMissExactly(t *testing.T) {
	var owner identity.Owner = fakeOwner{
		module:   "agency",
		instance: "agy1f2e3d4c",
		rows:     map[string]string{"role/uuid-1": "R-3"},
	}
	ctx := context.Background()

	raw, err := owner.StableIdentity(ctx, "role", "uuid-1")
	if err != nil {
		t.Fatalf("stable identity: %v", err)
	}
	if raw != "agency/agy1f2e3d4c/R-3" {
		t.Fatalf("raw = %q", raw)
	}
	object, id, err := owner.Resolve(ctx, raw)
	if err != nil || object != "role" || id != "uuid-1" {
		t.Fatalf("resolve = %q, %q, %v", object, id, err)
	}

	if _, err := owner.StableIdentity(ctx, "role", "uuid-9"); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("an unknown row: want ErrNotFound, got %v", err)
	}
	if _, _, err := owner.Resolve(ctx, "agency/agy1f2e3d4c/R-9"); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("a gone identity: want ErrNotFound, got %v", err)
	}
	if _, _, err := owner.Resolve(ctx, "git/agy1f2e3d4c/R-3"); !errors.Is(err, identity.ErrNotOwned) {
		t.Fatalf("another module's raw: want ErrNotOwned, got %v", err)
	}
	if _, _, err := owner.Resolve(ctx, "agency/R-3"); !errors.Is(err, identity.ErrInvalidRaw) {
		t.Fatalf("a malformed raw: want ErrInvalidRaw, got %v", err)
	}
}
