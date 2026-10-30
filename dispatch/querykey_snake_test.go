package dispatch_test

import (
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

func TestQueryKeyFallbackSpellsFieldsAsSpecstoreSpellsColumns(t *testing.T) {
	for _, field := range []string{"Name", "RoleCode", "OptionIDs", "OptionIDsX", "HTTPStatus", "ID", "Line2Item", "CreatedAt", "URLs"} {
		if got, want := dispatch.SnakeCase(field), specstore.ColumnName(field); got != want {
			t.Errorf("SnakeCase(%q) = %q, specstore.ColumnName = %q", field, got, want)
		}
	}
}
