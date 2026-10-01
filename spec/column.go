package spec

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func ColumnIsTextual(db *gorm.DB, table, column string) (bool, error) {
	var types []string
	q := `select type from pragma_table_info(?) where name = ?`
	if db.Dialector.Name() == "postgres" {
		q = `select data_type from information_schema.columns
			where table_schema = current_schema() and table_name = ? and column_name = ?`
	}
	if err := db.Raw(q, table, column).Scan(&types).Error; err != nil {
		return false, fmt.Errorf("spec: read the type of %s.%s: %w", table, column, err)
	}
	if len(types) == 0 {
		return false, fmt.Errorf("spec: %s has no column %s", table, column)
	}
	return textualColumnType(types[0]), nil
}

func DeclaresTextualColumn(o Object, column, dialect string) bool {
	for _, f := range o.Fields {
		if f.Name == column {
			return textualColumnType(f.columnType(dialect))
		}
	}
	return true
}

func textualColumnType(t string) bool {
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "" {
		return true
	}
	for _, mark := range []string{"text", "char", "clob", "string"} {
		if strings.Contains(t, mark) {
			return true
		}
	}
	return false
}

func UnsetSQL(column string, textual bool) string {
	if textual {
		return "(" + column + " IS NULL OR " + column + " = '')"
	}
	return "(" + column + " IS NULL)"
}

func UnsetClause(db *gorm.DB, o Object, table, column string) string {
	textual, err := ColumnIsTextual(db, table, column)
	if err != nil {
		return UnsetSQL(column, DeclaresTextualColumn(o, column, db.Dialector.Name()))
	}
	return UnsetSQL(column, textual)
}
