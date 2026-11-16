package specstore

import (
	"context"
	"gorm.io/gorm"
)

func (st *Store) InTransaction(ctx context.Context, fn func(*Store) error) error {
	return st.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Store{db: tx, spec: st.spec, objects: st.objects, runID: st.runID, listCap: st.listCap})
	})
}
