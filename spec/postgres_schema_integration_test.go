//go:build postgres

package spec_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func TestS39_PostgresAdoptionRefusesATimestamptzColumnAndRenamesNothing(t *testing.T) {
	db := postgresDB(t)
	s, err := spec.Parse([]byte(strings.Replace(adoptTypesSpec, `"instance": "platform"`, fmt.Sprintf(`"instance": "adopt%d"`, os.Getpid()), 1)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	legacy := spec.LegacyTables(s, s.Objects[0])[0]
	declared := s.TableFor(s.Objects[0])
	t.Cleanup(func() { db.Exec("drop table if exists " + legacy); db.Exec("drop table if exists " + declared) })
	if err := db.Exec(`create table ` + legacy + ` (id text primary key, body text, sent_at timestamptz, size bigint, score double precision, seen boolean, raw bytea, doc jsonb)`).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	err = spec.AdoptLegacy(db, s, spec.Legacy{})
	if err == nil || !strings.Contains(err.Error(), "sent_at is timestamp with time zone") {
		t.Fatalf("want a refusal naming sent_at, got %v", err)
	}
	if !db.Migrator().HasTable(legacy) || db.Migrator().HasTable(declared) {
		t.Fatal("the refusal came after the rename")
	}
}

func TestS53_PostgresRelaxesADroppedRequiredColumn(t *testing.T) {
	db := postgresDB(t)
	inst := fmt.Sprintf("orphan%d", os.Getpid())
	s1, err := spec.Parse([]byte(strings.Replace(orphanSpecV1, `"instance": "orphans"`, `"instance": "`+inst+`"`, 1)))
	if err != nil {
		t.Fatalf("parse v1: %v", err)
	}
	s2, err := spec.Parse([]byte(strings.Replace(orphanSpecV2, `"instance": "orphans"`, `"instance": "`+inst+`"`, 1)))
	if err != nil {
		t.Fatalf("parse v2: %v", err)
	}
	table := s1.TableFor(s1.Objects[0])
	test := s1.TestTableFor(s1.Objects[0])
	t.Cleanup(func() { db.Exec("drop table if exists " + table); db.Exec("drop table if exists " + test) })

	if err := spec.Migrate(db, s1); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	if err := db.Exec("insert into " + table + " (org, id, code, color, weight) values ('o', 'w1', 'c1', 'red', 5)").Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := spec.Migrate(db, s2); err != nil {
		t.Fatalf("migrate v2: %v", err)
	}
	if err := db.Exec("insert into " + table + " (org, id, code) values ('o', 'w2', 'c2')").Error; err != nil {
		t.Fatalf("a write naming only the declared columns failed: %v", err)
	}
	if err := db.Exec("insert into " + test + " (run_id, org, id, code) values ('r', 'o', 'w1', 'c1')").Error; err != nil {
		t.Fatalf("the test copy still blocks: %v", err)
	}
	var color string
	if err := db.Raw("select color from " + table + " where id = 'w1'").Scan(&color).Error; err != nil || color != "red" {
		t.Fatalf("the dropped column lost its data: %q %v", color, err)
	}
}
