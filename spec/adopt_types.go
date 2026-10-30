package spec

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

var typeFamilies = map[string]string{
	"boolean": "bool", "bool": "bool",
	"smallint": "int", "integer": "int", "bigint": "int", "int2": "int", "int4": "int", "int8": "int",
	"real": "float", "double precision": "float", "float4": "float", "float8": "float", "numeric": "float", "decimal": "float",
	"bytea": "bytes", "blob": "bytes",
	"json": "json", "jsonb": "json",
	"text": "text", "character varying": "text", "varchar": "text", "character": "text", "char": "text", "bpchar": "text",
}

func typeFamily(dbType string) string {
	t := strings.ToLower(strings.TrimSpace(dbType))
	if family, ok := typeFamilies[t]; ok {
		return family
	}
	return t
}

func heldColumnFor(field string, held map[string]string, columns map[string]string) (string, bool) {
	if _, ok := held[field]; ok {
		return field, true
	}
	for legacy, want := range columns {
		if want != field {
			continue
		}
		if _, ok := held[legacy]; ok {
			return legacy, true
		}
	}
	return "", false
}

func adoptionTypeProblems(table string, held map[string]string, o Object, dialect string, columns map[string]string) []string {
	var problems []string
	for _, f := range o.Fields {
		column, ok := heldColumnFor(f.Name, held, columns)
		if !ok {
			continue
		}
		want := f.columnType(dialect)
		if typeFamily(held[column]) == typeFamily(want) {
			continue
		}
		problems = append(problems, fmt.Sprintf("%s.%s is %s and the spec declares %s as %s",
			table, column, held[column], f.Name, want))
	}
	return problems
}

func columnTypes(db *gorm.DB, table string) (map[string]string, error) {
	var rows []struct {
		Name string
		Type string
	}
	q := `select column_name as name, data_type as type from information_schema.columns
		where table_schema = current_schema() and table_name = ?`
	if err := db.Raw(q, table).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("spec: read the column types of %s: %w", table, err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Name] = r.Type
	}
	return out, nil
}

func checkAdoptedColumnTypes(db *gorm.DB, s *Spec, l Legacy) error {
	dialect := db.Dialector.Name()
	if dialect != "postgres" {
		return nil
	}
	m := db.Migrator()
	var problems []string
	for _, o := range s.Objects {
		if m.HasTable(s.TableFor(o)) {
			continue
		}
		for _, legacy := range legacyTablesWith(s, o, l) {
			if !m.HasTable(legacy) {
				continue
			}
			held, err := columnTypes(db, legacy)
			if err != nil {
				return err
			}
			problems = append(problems, adoptionTypeProblems(legacy, held, o, dialect, l.Columns)...)
			break
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("spec: adopt: refusing to rename legacy tables whose column types the engine cannot read: %s; retype each column by hand (alter table ... alter column ... type ... using ...) and adopt again",
		strings.Join(problems, "; "))
}
