package spec

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type Legacy struct {
	Columns map[string]string

	IndexPrefixes []string

	IndexSuffixes []string
}

func LegacyTables(s *Spec, o Object) []string {
	object := o.table()
	declared := s.TableFor(o)

	candidates := []string{s.Instance + "_" + s.Module + "_" + s.MountName() + "_" + object}
	if s.MountName() == DefaultMount {
		candidates = append(candidates,
			s.Instance+"_"+s.Module+"_"+object,
			s.Instance+"_"+object,
		)
	}

	out := make([]string, 0, len(candidates))
	for _, name := range candidates {
		if name != declared {
			out = append(out, name)
		}
	}
	return out
}

func Adopted(db *gorm.DB, s *Spec) (bool, error) {
	declared := make([]string, 0, len(s.Objects))
	legacy := make([]string, 0, len(s.Objects))
	for _, o := range s.Objects {
		declared = append(declared, s.TableFor(o))
		legacy = append(legacy, LegacyTables(s, o)...)
	}

	held, err := TablesNamed(db, append(append([]string{}, declared...), legacy...))
	if err != nil {
		return false, err
	}
	for _, name := range declared {
		if !held[name] {
			return false, nil
		}
	}
	for _, name := range legacy {
		if held[name] {
			return false, nil
		}
	}
	return true, nil
}

func AdoptLegacy(db *gorm.DB, s *Spec, l Legacy) error {
	m := db.Migrator()
	for _, o := range s.Objects {
		declared := s.TableFor(o)

		for _, legacy := range LegacyTables(s, o) {
			if !m.HasTable(legacy) {
				continue
			}
			if m.HasTable(declared) {
				stranded, err := RowCount(db, legacy)
				if err != nil {
					return err
				}
				if stranded > 0 {
					return fmt.Errorf("spec: %s and %s both exist and %s still holds %d row(s), which no read would ever reach again",
						legacy, declared, legacy, stranded)
				}
				continue
			}
			if err := db.Exec(fmt.Sprintf("alter table %s rename to %s", legacy, declared)).Error; err != nil {
				return fmt.Errorf("spec: rename %s to %s: %w", legacy, declared, err)
			}
			if err := DropIndexes(db, declared, l); err != nil {
				return err
			}
		}

		if !m.HasTable(declared) {
			continue
		}
		if err := renameColumns(db, declared, o, l.Columns); err != nil {
			return err
		}
	}
	return nil
}

func renameColumns(db *gorm.DB, table string, o Object, columns map[string]string) error {
	for legacy, want := range columns {
		if !DeclaresColumn(o, want) {
			continue
		}
		held, err := HasColumn(db, table, legacy)
		if err != nil {
			return err
		}
		already, err := HasColumn(db, table, want)
		if err != nil {
			return err
		}
		if !held || already {
			continue
		}
		stmt := fmt.Sprintf("alter table %s rename column %s to %s", table, legacy, want)
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("spec: rename %s.%s: %w", table, legacy, err)
		}
	}
	return nil
}

func DropIndexes(db *gorm.DB, table string, l Legacy) error {
	var names []string
	switch db.Dialector.Name() {
	case "postgres":
		if err := db.Raw(`select indexname from pg_indexes where tablename = ?`, table).
			Scan(&names).Error; err != nil {
			return fmt.Errorf("spec: read indexes of %s: %w", table, err)
		}
	default:
		if err := db.Raw(`select name from sqlite_master where type = 'index' and tbl_name = ?`, table).
			Scan(&names).Error; err != nil {
			return fmt.Errorf("spec: read indexes of %s: %w", table, err)
		}
	}

	for _, name := range names {
		if !legacyIndex(name, l) {
			continue
		}
		if err := db.Exec("drop index if exists " + name).Error; err != nil {
			return fmt.Errorf("spec: drop index %s: %w", name, err)
		}
	}
	return nil
}

func legacyIndex(name string, l Legacy) bool {
	prefixes := l.IndexPrefixes
	if prefixes == nil {
		prefixes = []string{"idx_"}
	}
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	for _, s := range l.IndexSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

func TablesNamed(db *gorm.DB, names []string) (map[string]bool, error) {
	if len(names) == 0 {
		return map[string]bool{}, nil
	}
	query := `select table_name from information_schema.tables
		where table_schema = current_schema() and table_name in ?`
	if db.Dialector.Name() != "postgres" {
		query = `select name from sqlite_master where type = 'table' and name in ?`
	}
	var found []string
	if err := db.Raw(query, names).Scan(&found).Error; err != nil {
		return nil, fmt.Errorf("spec: read the table set: %w", err)
	}
	held := make(map[string]bool, len(found))
	for _, name := range found {
		held[name] = true
	}
	return held, nil
}

func RowCount(db *gorm.DB, table string) (int64, error) {
	var n int64
	if err := db.Table(table).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("spec: count %s: %w", table, err)
	}
	return n, nil
}

func HasColumn(db *gorm.DB, table, column string) (bool, error) {
	var found []int
	q := `select 1 from pragma_table_info(?) where name = ?`
	if db.Dialector.Name() == "postgres" {
		q = `select 1 from information_schema.columns where table_name = ? and column_name = ?`
	}
	if err := db.Raw(q, table, column).Scan(&found).Error; err != nil {
		return false, fmt.Errorf("spec: read columns of %s: %w", table, err)
	}
	return len(found) > 0, nil
}

func DeclaresColumn(o Object, column string) bool {
	for _, f := range o.Fields {
		if f.Name == column {
			return true
		}
	}
	return false
}
