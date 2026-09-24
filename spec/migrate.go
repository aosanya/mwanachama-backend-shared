package spec

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Migrate creates a table for every declared object, and its indexes.
//
// It is the spec's whole storage story: there are no row structs and no
// AutoMigrate, because the objects are data. Running it twice is a no-op.
func Migrate(db *gorm.DB, s *Spec) error {
	dialect := db.Dialector.Name()
	for _, stmt := range s.DDL(dialect) {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("spec: migrate: %w\n  in: %s", err, stmt)
		}
	}
	return nil
}

// DDL returns every statement Migrate would run, in order. Having it
// separately is what lets a spec be reviewed as SQL before it touches a
// database, and what lets a test assert on the statements rather than on
// their effects.
func (s *Spec) DDL(dialect string) []string {
	var out []string
	for _, o := range s.Objects {
		out = append(out, s.createTable(o, dialect))
		out = append(out, s.createIndexes(o, dialect)...)
	}
	return out
}

func (s *Spec) createTable(o Object, dialect string) string {
	table := s.TableFor(o)

	var cols []string
	var primaries []string
	for _, f := range o.Fields {
		cols = append(cols, "  "+f.column(dialect))
		if f.Primary {
			primaries = append(primaries, f.Name)
		}
	}
	// A single primary is declared on the column; a composite needs its own
	// clause, and declaring it that way for both keeps one code path.
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
	case TypeBool:
		return "boolean"
	case TypeJSON:
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
		if f.Type == TypeInt || f.Type == TypeBool {
			return "default " + f.Default
		}
		return "default '" + strings.ReplaceAll(f.Default, "'", "''") + "'"

	// A required field with a default is a contradiction: the default is
	// exactly what stops an omitted value being noticed. The convenience
	// defaults below are for optional columns only.
	case f.Required, f.Primary, f.Type == TypeJSON:
		return ""
	case f.Type == TypeInt:
		return "default 0"
	case f.Type == TypeBool:
		return "default false"
	default:
		return "default ''"
	}
}

func (s *Spec) createIndexes(o Object, dialect string) []string {
	table := s.TableFor(o)
	var out []string
	for _, idx := range o.Indexes {
		name := s.IndexFor(o, idx)

		unique := ""
		if idx.Unique {
			unique = "unique "
		}

		var on string
		switch {
		case idx.Path != nil:
			// Postgres needs the expression in its own parentheses inside
			// the index's — `on t ((expr))`, not `on t (expr)`, which is a
			// syntax error. SQLite accepts one pair, so a unit test on
			// SQLite alone will not catch a missing one.
			on = "((" + docPathExpr(idx.Path.Field, idx.Path.Path, dialect) + "))"
		default:
			on = "(" + strings.Join(idx.Fields, ", ") + ")"
		}

		where := ""
		if idx.NotDeleted {
			where = " where deleted = false"
		}

		out = append(out, fmt.Sprintf("create %sindex if not exists %s on %s %s%s",
			unique, name, table, on, where))
	}
	return out
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
