package gormtest_test

import (
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/gormtest"
)

type widget struct {
	ID   uint `gorm:"primarykey"`
	Name string
}

func TestOpenSQLiteDB(t *testing.T) {
	db := gormtest.OpenSQLiteDB(t)

	if err := db.AutoMigrate(&widget{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&widget{Name: "gear"}).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}

	var got widget
	if err := db.First(&got, "name = ?", "gear").Error; err != nil {
		t.Fatalf("First: %v", err)
	}
	if got.Name != "gear" {
		t.Errorf("Name = %q, want %q", got.Name, "gear")
	}
}

func TestOpenPostgresDB_SkipsWithoutPOSTGRES_URL(t *testing.T) {
	t.Setenv("POSTGRES_URL", "")
	gormtest.OpenPostgresDB(t, "gormtestit")
	t.Fatal("expected OpenPostgresDB to skip when POSTGRES_URL is unset")
}
