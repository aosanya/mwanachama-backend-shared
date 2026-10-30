package specstore

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

var ErrRunIDEmpty = errors.New("specstore: a run id must not be empty")

func (st *Store) ForRun(runID string) (*Store, error) {
	if runID == "" {
		return nil, ErrRunIDEmpty
	}
	scoped := &Store{db: st.db, spec: st.spec, objects: st.objects, runID: runID, listCap: st.listCap}
	return scoped, nil
}

func (st *Store) RunID() string { return st.runID }

func (st *Store) Row(role string, v any) (map[string]any, error) {
	row, err := Encode(st.Object(role), v)
	if err != nil {
		return nil, err
	}
	if st.runID != "" {
		row[spec.RunIDColumn] = st.runID
	}
	return row, nil
}

func RemoveRun(ctx context.Context, db *gorm.DB, s *spec.Spec, runID string) (int64, error) {
	if runID == "" {
		return 0, ErrRunIDEmpty
	}
	var removed int64
	for _, o := range s.Objects {
		if !db.Migrator().HasTable(s.TestTableFor(o)) {
			continue
		}
		res := db.WithContext(ctx).Table(s.TestTableFor(o)).Where(spec.RunIDColumn+" = ?", runID).Delete(nil)
		if res.Error != nil {
			return removed, fmt.Errorf("specstore: remove run %s from %s: %w", runID, s.TestTableFor(o), res.Error)
		}
		removed += res.RowsAffected
	}
	return removed, nil
}

type RunRows struct {
	Object string
	Table  string
	Rows   int64
}

func RowsOfRun(ctx context.Context, db *gorm.DB, s *spec.Spec, runID string) ([]RunRows, error) {
	if runID == "" {
		return nil, ErrRunIDEmpty
	}
	var out []RunRows
	for _, o := range s.Objects {
		table := s.TestTableFor(o)
		if !db.Migrator().HasTable(table) {
			continue
		}
		var n int64
		if err := db.WithContext(ctx).Table(table).Where(spec.RunIDColumn+" = ?", runID).Count(&n).Error; err != nil {
			return nil, fmt.Errorf("specstore: count run %s in %s: %w", runID, table, err)
		}
		if n > 0 {
			out = append(out, RunRows{Object: o.Name, Table: table, Rows: n})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Table < out[j].Table })
	return out, nil
}
