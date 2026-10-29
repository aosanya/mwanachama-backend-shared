package spec

import (
	"fmt"

	"gorm.io/gorm"
)

func (s *Spec) AlterDDL(dialect string, existing map[string][]string) ([]string, error) {
	var out []string
	for _, o := range s.storedObjects() {
		held, there := existing[s.TableFor(o)]
		if !there {
			continue
		}
		stmts, missingKeys, err := s.alterObject(o, dialect, held, false)
		if err != nil {
			return nil, err
		}
		if len(missingKeys) > 0 {
			return nil, missingKeyError(s.TableFor(o), missingKeys[0])
		}
		out = append(out, stmts...)
	}
	for _, o := range s.Objects {
		held, there := existing[s.TestTableFor(o)]
		if !there {
			continue
		}
		stmts, _, err := s.alterObject(o, dialect, held, true)
		if err != nil {
			return nil, err
		}
		out = append(out, stmts...)
	}
	return out, nil
}

func missingKeyError(table, column string) error {
	return fmt.Errorf("spec: migrate: table %s exists without its primary key column %s, and a primary key column cannot be added to a table; add the key by hand",
		table, column)
}

func (s *Spec) alterObject(o Object, dialect string, held []string, test bool) (stmts []string, missingKeys []string, err error) {
	table := s.physicalTable(o, test)
	have := make(map[string]bool, len(held))
	for _, name := range held {
		have[name] = true
	}

	for _, f := range o.Fields {
		if have[f.Name] {
			continue
		}
		if f.Primary {
			missingKeys = append(missingKeys, f.Name)
			continue
		}
		stmts = append(stmts, fmt.Sprintf("alter table %s add column %s", table, f.addedColumn(dialect)))
		if f.Unique {
			index := table + "_" + f.Name + "_uq"
			if len(index) > MaxIdentifier {
				return nil, nil, fmt.Errorf("spec: migrate: the unique index for new column %s of %s needs %d bytes as %q, and the database truncates at %d",
					f.Name, table, len(index), index, MaxIdentifier)
			}
			columns := f.Name
			if test {
				columns = RunIDColumn + ", " + f.Name
			}
			stmts = append(stmts, fmt.Sprintf("create unique index if not exists %s on %s (%s)", index, table, columns))
		}
	}
	return stmts, missingKeys, nil
}

func (f Field) addedColumn(dialect string) string {
	f.Unique = false
	if !f.Required || f.Default != "" {
		return f.column(dialect)
	}
	empty, hasEmpty := emptyValueSQL(f.Type)
	if !hasEmpty {
		f.Required = false
		return f.column(dialect)
	}
	return f.column(dialect) + " default " + empty
}

func emptyValueSQL(t FieldType) (string, bool) {
	switch t {
	case TypeInt, TypeFloat:
		return "0", true
	case TypeBool:
		return "false", true
	case TypeJSON, TypeGeometry:
		return "", false
	default:
		return "''", true
	}
}

func evolveTable(db *gorm.DB, s *Spec, o Object, dialect string, test bool) error {
	table := s.physicalTable(o, test)
	held, err := columnNames(db, table)
	if err != nil {
		return err
	}
	stmts, _, err := s.alterObject(o, dialect, held, test)
	if err != nil || len(stmts) == 0 {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, stmt := range stmts {
			if err := tx.Exec(stmt).Error; err != nil {
				return fmt.Errorf("spec: migrate: %w\n  in: %s", err, stmt)
			}
		}
		return nil
	})
}

func columnNames(db *gorm.DB, table string) ([]string, error) {
	q := `select name from pragma_table_info(?)`
	if db.Dialector.Name() == "postgres" {
		q = `select column_name from information_schema.columns
			where table_schema = current_schema() and table_name = ?`
	}
	var names []string
	if err := db.Raw(q, table).Scan(&names).Error; err != nil {
		return nil, fmt.Errorf("spec: migrate: read the columns of %s: %w", table, err)
	}
	return names, nil
}
