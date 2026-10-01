package specstore_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const roleKey = "key"

const keySpec = `{
  "module": "vault",
  "domain": "deployment",
  "instance": "dep1",
  "objects": [
    {
      "name": "key",
      "table": "keys",
      "role": "key",
      "description": "One stored key.",
      "fields": [
        {"name": "id", "type": "int", "description": "Which version of the key this is.", "primary": true},
        {"name": "secret", "type": "bytes", "description": "The raw key material."},
        {"name": "label", "type": "string", "description": "What the key is for."}
      ]
    }
  ]
}`

type Key struct {
	ID     int    `json:"id"`
	Secret []byte `json:"-"`
	Label  string `json:"label"`
}

var errNoKey = errors.New("no such key")

func newKeyStore(t *testing.T) *specstore.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s, err := spec.Parse([]byte(keySpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st, err := specstore.New(db, s, map[string]any{roleKey: Key{}})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return st
}

func TestBytesRoundTripNonUTF8KeyMaterial(t *testing.T) {
	st := newKeyStore(t)
	ctx := context.Background()

	raw := []byte{0x00, 0xff, 0xfe, 0x80, 0x01, 0x7f, 0xc0, 0x80}
	if !bytes.Contains(raw, []byte{0xff}) {
		t.Fatal("the fixture must hold a byte no UTF-8 text column could carry")
	}
	if err := st.Insert(ctx, roleKey, Key{ID: 1, Secret: raw, Label: "phone hashing"}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var got Key
	if err := st.Take(st.Query(ctx, roleKey).Where("id = ?", 1), roleKey, &got, errNoKey); err != nil {
		t.Fatalf("take: %v", err)
	}
	if !bytes.Equal(got.Secret, raw) {
		t.Fatalf("secret round-tripped as % x, want % x", got.Secret, raw)
	}
	if got.Label != "phone hashing" {
		t.Fatalf("label = %q", got.Label)
	}
}

func TestBytesColumnIsNotTextOnPostgres(t *testing.T) {
	s, err := spec.Parse([]byte(keySpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ddl := strings.Join(s.DDL("postgres"), "\n")
	if !strings.Contains(ddl, "bytea") {
		t.Fatalf("a declared bytes field must reach Postgres as bytea, got:\n%s", ddl)
	}
	if strings.Contains(ddl, "secret text") {
		t.Fatalf("the secret column reached Postgres as text:\n%s", ddl)
	}
}

func TestBytesRefusesAStringCarrier(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s, err := spec.Parse([]byte(keySpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	type stringCarrier struct {
		ID     int    `json:"id"`
		Secret string `json:"-"`
		Label  string `json:"label"`
	}
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st, err := specstore.New(db, s, map[string]any{roleKey: stringCarrier{}})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	err = st.Insert(context.Background(), roleKey, stringCarrier{ID: 1, Secret: "not bytes"})
	if err == nil {
		t.Fatal("a declared bytes column accepted a string carrier; the placeholder-write class of bug is back")
	}
}

func TestBytesCannotBeAKeyOrCarryADefault(t *testing.T) {
	for _, tc := range []struct{ name, field string }{
		{"primary", `{"name": "secret", "type": "bytes", "description": "d", "primary": true}`},
		{"default", `{"name": "secret", "type": "bytes", "description": "d", "default": "x"}`},
		{"matches", `{"name": "secret", "type": "bytes", "description": "d", "matches": "slug"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{"module":"vault","domain":"d","instance":"i","objects":[{"name":"key","table":"keys","role":"key","description":"d","fields":[{"name":"id","type":"int","description":"d","primary":true},` + tc.field + `]}]}`
			if _, err := spec.Parse([]byte(raw)); err == nil {
				t.Fatalf("a bytes field declaring %s was accepted", tc.name)
			}
		})
	}
}
