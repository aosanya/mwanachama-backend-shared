package spec

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB, s *Spec) error {
	if problems := s.defaultProblems(); len(problems) > 0 {
		return asError("spec: migrate", problems)
	}
	dialect := db.Dialector.Name()
	for _, o := range s.storedObjects() {
		if err := migrateTable(db, s, o, dialect, false); err != nil {
			return err
		}
	}
	for _, o := range s.Objects {
		if err := migrateTable(db, s, o, dialect, true); err != nil {
			return err
		}
	}
	return RecordNames(db, s)
}

func migrateTable(db *gorm.DB, s *Spec, o Object, dialect string, test bool) error {
	if err := execDDL(db, s.createTable(o, dialect, test)); err != nil {
		return err
	}
	if err := evolveTable(db, s, o, dialect, test); err != nil {
		return err
	}
	for _, stmt := range s.createIndexes(o, dialect, test) {
		if err := execDDL(db, stmt); err != nil {
			return err
		}
	}
	return nil
}

func execDDL(db *gorm.DB, stmt string) error {
	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("spec: migrate: %w\n  in: %s", err, stmt)
	}
	return nil
}

func (s *Spec) DDL(dialect string) []string {
	var out []string
	for _, o := range s.storedObjects() {
		out = append(out, s.createTable(o, dialect, false))
		out = append(out, s.createIndexes(o, dialect, false)...)
	}
	for _, o := range s.Objects {
		out = append(out, s.createTable(o, dialect, true))
		out = append(out, s.createIndexes(o, dialect, true)...)
	}
	return out
}

func (s *Spec) physicalTable(o Object, test bool) string {
	if test {
		return s.TestTableFor(o)
	}
	return s.TableFor(o)
}

func runIDColumn() string {
	return RunIDColumn + " text not null"
}

func (s *Spec) createTable(o Object, dialect string, test bool) string {
	table := s.physicalTable(o, test)

	var cols []string
	var primaries []string
	if test {
		cols = append(cols, "  "+runIDColumn())
		primaries = append(primaries, RunIDColumn)
	}
	for _, f := range o.Fields {
		if test {
			f.Unique = false
		}
		cols = append(cols, "  "+f.column(dialect))
		if f.Primary {
			primaries = append(primaries, f.Name)
		}
	}
	cols = append(cols, "  primary key ("+strings.Join(primaries, ", ")+")")

	return fmt.Sprintf("create table if not exists %s (\n%s\n)", table, strings.Join(cols, ",\n"))
}

func (f Field) column(dialect string) string {
	parts := []string{f.Name, f.columnType(dialect)}

	// Required means the write is rejected, which is the module's job. The
	// column is still NOT NULL so a row that got in another way is wrong in
	// the database too, with a default to make that possible.
	if f.Required || f.Primary {
		parts = append(parts, "not null")
	}
	if d := f.defaultClause(); d != "" {
		parts = append(parts, d)
	}
	if f.Unique && !f.Primary {
		parts = append(parts, "unique")
	}
	return strings.Join(parts, " ")
}

func (f Field) columnType(dialect string) string {
	switch f.Type {
	case TypeInt:
		return "bigint"
	case TypeFloat:
		return "double precision"
	case TypeBool:
		return "boolean"
	case TypeBytes:
		if dialect == "postgres" {
			return "bytea"
		}
		return "blob"
	case TypeJSON, TypeGeometry:
		// The split that lets the unit tests run on SQLite while the real
		// document queries run on jsonb. SQLite has JSON1, so json_extract
		// works over text there.
		if dialect == "postgres" {
			return "jsonb"
		}
		return "text"
	default:
		// string, text, timestamp and enum are all text: an enum's values
		// are enforced by the module, deliberately not by a CHECK nobody
		// re-reads.
		return "text"
	}
}

func (f Field) defaultClause() string {
	switch {
	case f.Default != "":
		if literal, err := f.defaultLiteral(); err == nil {
			return "default " + literal
		}
		return "default " + quotedLiteral(f.Default)

	// A required field with a default is a contradiction: the default is
	// exactly what stops an omitted value being noticed. The convenience
	// defaults below are for optional columns only. A nullable column is
	// excluded for the opposite reason: a default is what would stop NULL
	// ever being stored, which is the whole point of declaring it nullable.
	case f.Required, f.Primary, f.Nullable, f.Type == TypeJSON, f.Type == TypeGeometry:
		return ""
	case f.Type == TypeInt, f.Type == TypeFloat:
		return "default 0"
	case f.Type == TypeBool:
		return "default false"
	default:
		return "default ''"
	}
}

func (s *Spec) createIndexes(o Object, dialect string, test bool) []string {
	table := s.physicalTable(o, test)
	var out []string
	if test {
		out = append(out, s.testUniqueFieldIndexes(o)...)
	}
	for _, idx := range o.Indexes {
		name := s.IndexFor(o, idx)
		if test {
			name = s.TestIndexFor(o, idx)
		}

		unique := ""
		if idx.Unique {
			unique = "unique "
		}

		var on string
		switch {
		case idx.Path != nil && test:
			on = "(" + RunIDColumn + ", (" + docPathExpr(idx.Path.Field, idx.Path.Path, dialect) + "))"
		case idx.Path != nil:
			on = "((" + docPathExpr(idx.Path.Field, idx.Path.Path, dialect) + "))"
		case test:
			on = "(" + strings.Join(append([]string{RunIDColumn}, idx.Fields...), ", ") + ")"
		default:
			on = "(" + strings.Join(idx.Fields, ", ") + ")"
		}

		where := ""
		if conds := idx.conditions(); len(conds) > 0 {
			where = " where " + strings.Join(conds, " and ")
		}

		out = append(out, fmt.Sprintf("create %sindex if not exists %s on %s %s%s",
			unique, name, table, on, where))
	}
	return out
}

func (s *Spec) testUniqueFieldIndexes(o Object) []string {
	table := s.TestTableFor(o)
	var out []string
	for _, f := range o.Fields {
		if f.Unique && !f.Primary {
			out = append(out, fmt.Sprintf("create unique index if not exists %s on %s (%s, %s)",
				testUniqueIndexName(table, f.Name), table, RunIDColumn, f.Name))
		}
	}
	return out
}

func testUniqueIndexName(table, field string) string {
	return table + "_" + field + "_uq"
}

// DocPathExpr builds the SQL that reads one path out of a json column, in
// the dialect in use. Both the index and the query filter go through here,
// so an index is always on the expression the filter actually uses.
//
// Neither dialect takes the path as a bound parameter, which is why the spec
// validates it against [DocPathPattern] when it loads.
func DocPathExpr(field, path, dialect string) string {
	return docPathExpr(field, path, dialect)
}

func docPathExpr(field, path, dialect string) string {
	segs := strings.Split(path, ".")
	if dialect == "postgres" {
		return field + " #>> '{" + strings.Join(segs, ",") + "}'"
	}
	return "json_extract(" + field + ", '$." + strings.Join(segs, ".") + "')"
}
