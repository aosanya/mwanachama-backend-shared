package spec

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NameRecord struct {
	Physical string `gorm:"column:physical;primaryKey"`
	MountKey string `gorm:"column:mount_key"`
	Instance string `gorm:"column:instance"`
	Module   string `gorm:"column:module"`
	Mount    string `gorm:"column:mount"`
	Object   string `gorm:"column:object"`
	Raw      string `gorm:"column:raw"`
}

func (s *Spec) NameRecords() []NameRecord {
	out := make([]NameRecord, 0, len(s.Objects))
	for _, o := range s.Objects {
		out = append(out, NameRecord{
			Physical: s.TableFor(o),
			MountKey: s.MountKey(),
			Instance: s.Instance,
			Module:   s.Module,
			Mount:    s.MountName(),
			Object:   o.table(),
			Raw:      s.RawNameFor(o),
		})
	}
	return out
}

var nameRegistryColumns = []string{"physical", "mount_key", "instance", "module", "mount", "object", "raw"}

func NameRegistryDDL(instance string) []string {
	table := NameRegistryTableFor(instance)
	return []string{
		fmt.Sprintf(`create table if not exists %s (
  physical text not null,
  mount_key text not null,
  instance text not null,
  module text not null,
  mount text not null,
  object text not null,
  raw text not null,
  primary key (physical)
)`, table),
		fmt.Sprintf(`create unique index if not exists %s_raw_idx on %s (raw)`, table, table),
		fmt.Sprintf(`create index if not exists %s_mount_idx on %s (mount_key)`, table, table),
	}
}

func ensureNameRegistry(db *gorm.DB, instance string) error {
	table := NameRegistryTableFor(instance)
	create := NameRegistryDDL(instance)[0]
	if err := db.Exec(create).Error; err != nil {
		return fmt.Errorf("spec: name registry: %w\n  in: %s", err, create)
	}
	held := map[string]bool{}
	columns, err := db.Migrator().ColumnTypes(table)
	if err != nil {
		return fmt.Errorf("spec: name registry: %w", err)
	}
	for _, c := range columns {
		held[c.Name()] = true
	}
	for _, column := range nameRegistryColumns {
		if held[column] {
			continue
		}
		stmt := fmt.Sprintf("alter table %s add column %s text not null default ''", table, column)
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("spec: name registry: %w\n  in: %s", err, stmt)
		}
	}
	for _, stmt := range NameRegistryDDL(instance)[1:] {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("spec: name registry: %w\n  in: %s", err, stmt)
		}
	}
	return nil
}

func RecordNames(db *gorm.DB, s *Spec) error {
	if err := ensureNameRegistry(db, s.Instance); err != nil {
		return err
	}

	records := s.NameRecords()
	if len(records) == 0 {
		return nil
	}
	for _, rec := range records {
		err := db.Table(s.NameRegistryTable()).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "raw"}},
			DoUpdates: clause.AssignmentColumns([]string{"physical", "mount_key", "instance", "module", "mount", "object"}),
		}).Create(&rec).Error
		if err != nil {
			return fmt.Errorf("spec: name registry: %w", err)
		}
	}
	return nil
}

func LookupName(db *gorm.DB, physical string) (NameRecord, bool, error) {
	var rec NameRecord
	table := NameRegistryTableFor(InstanceOf(physical))
	err := db.Table(table).Where("physical = ?", physical).Take(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return NameRecord{}, false, nil
	}
	if err != nil {
		return NameRecord{}, false, fmt.Errorf("spec: name registry: %w", err)
	}
	return rec, true, nil
}
