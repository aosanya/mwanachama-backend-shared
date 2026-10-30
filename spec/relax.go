package spec

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func declaredColumns(o Object, test bool) map[string]bool {
	declared := make(map[string]bool, len(o.Fields)+1)
	for _, f := range o.Fields {
		declared[f.Name] = true
	}
	if test {
		declared[RunIDColumn] = true
	}
	return declared
}

func relaxStatements(table string, columns []string) []string {
	out := make([]string, 0, len(columns))
	for _, c := range columns {
		out = append(out, fmt.Sprintf("alter table %s alter column %s drop not null", table, quoteIdent(c)))
	}
	return out
}

func relaxOrphanColumns(db *gorm.DB, table string, declared map[string]bool) error {
	blocking, err := blockingColumns(db, table)
	if err != nil {
		return err
	}
	var orphans []string
	for _, c := range blocking {
		if !declared[c] {
			orphans = append(orphans, c)
		}
	}
	if len(orphans) == 0 {
		return nil
	}
	if db.Dialector.Name() == "postgres" {
		return db.Transaction(func(tx *gorm.DB) error {
			for _, stmt := range relaxStatements(table, orphans) {
				if err := tx.Exec(stmt).Error; err != nil {
					return fmt.Errorf("spec: migrate: %w\n  in: %s", err, stmt)
				}
			}
			return nil
		})
	}
	return rebuildNullable(db, table, orphans)
}

func blockingColumns(db *gorm.DB, table string) ([]string, error) {
	q := `select name from pragma_table_info(?) where "notnull" = 1 and dflt_value is null and pk = 0`
	if db.Dialector.Name() == "postgres" {
		q = `select c.column_name from information_schema.columns c
			where c.table_schema = current_schema() and c.table_name = ?
			  and c.is_nullable = 'NO' and c.column_default is null
			  and c.is_identity = 'NO' and c.is_generated = 'NEVER'
			  and c.column_name not in (
			    select a.attname from pg_index i
			    join pg_attribute a on a.attrelid = i.indrelid and a.attnum = any(i.indkey)
			    where i.indisprimary and i.indrelid = to_regclass(?))`
		var names []string
		if err := db.Raw(q, table, table).Scan(&names).Error; err != nil {
			return nil, fmt.Errorf("spec: migrate: read the constraints of %s: %w", table, err)
		}
		return names, nil
	}
	var names []string
	if err := db.Raw(q, table).Scan(&names).Error; err != nil {
		return nil, fmt.Errorf("spec: migrate: read the constraints of %s: %w", table, err)
	}
	return names, nil
}

type sqliteColumn struct {
	Name      string
	Type      string
	NotNull   int
	DfltValue *string
	Pk        int
}

func rebuildNullable(db *gorm.DB, table string, relax []string) error {
	relaxed := make(map[string]bool, len(relax))
	for _, c := range relax {
		relaxed[c] = true
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var cols []sqliteColumn
		if err := tx.Raw(`select name, type, "notnull" as not_null, dflt_value, pk from pragma_table_info(?) order by cid`, table).
			Scan(&cols).Error; err != nil {
			return fmt.Errorf("spec: migrate: read the columns of %s: %w", table, err)
		}
		uniques, err := sqliteUniques(tx, table)
		if err != nil {
			return err
		}
		var indexes []string
		if err := tx.Raw(`select sql from sqlite_master where type = 'index' and tbl_name = ? and sql is not null`, table).
			Scan(&indexes).Error; err != nil {
			return fmt.Errorf("spec: migrate: read the indexes of %s: %w", table, err)
		}

		var defs, keys, names []string
		keyAt := map[int]string{}
		for _, c := range cols {
			names = append(names, quoteIdent(c.Name))
			def := quoteIdent(c.Name)
			if c.Type != "" {
				def += " " + c.Type
			}
			if (c.NotNull == 1 && !relaxed[c.Name]) || c.Pk > 0 {
				def += " not null"
			}
			if c.DfltValue != nil {
				def += " default " + *c.DfltValue
			}
			defs = append(defs, def)
			if c.Pk > 0 {
				keyAt[c.Pk] = quoteIdent(c.Name)
			}
		}
		for i := 1; i <= len(keyAt); i++ {
			keys = append(keys, keyAt[i])
		}
		if len(keys) > 0 {
			defs = append(defs, "primary key ("+strings.Join(keys, ", ")+")")
		}
		defs = append(defs, uniques...)

		scratch := table + "__rebuild"
		steps := []string{
			fmt.Sprintf("create table %s (%s)", scratch, strings.Join(defs, ", ")),
			fmt.Sprintf("insert into %s (%s) select %s from %s", scratch, strings.Join(names, ", "), strings.Join(names, ", "), table),
			"drop table " + table,
			fmt.Sprintf("alter table %s rename to %s", scratch, table),
		}
		steps = append(steps, indexes...)
		for _, stmt := range steps {
			if err := tx.Exec(stmt).Error; err != nil {
				return fmt.Errorf("spec: migrate: %w\n  in: %s", err, stmt)
			}
		}
		return nil
	})
}

func sqliteUniques(tx *gorm.DB, table string) ([]string, error) {
	var held []string
	if err := tx.Raw(`select name from pragma_index_list(?) where origin = 'u' and "unique" = 1`, table).
		Scan(&held).Error; err != nil {
		return nil, fmt.Errorf("spec: migrate: read the unique constraints of %s: %w", table, err)
	}
	var out []string
	for _, index := range held {
		var cols []string
		if err := tx.Raw(`select name from pragma_index_info(?) order by seqno`, index).Scan(&cols).Error; err != nil {
			return nil, fmt.Errorf("spec: migrate: read the unique constraint %s: %w", index, err)
		}
		quoted := make([]string, len(cols))
		for i, c := range cols {
			quoted[i] = quoteIdent(c)
		}
		out = append(out, "unique ("+strings.Join(quoted, ", ")+")")
	}
	return out, nil
}
