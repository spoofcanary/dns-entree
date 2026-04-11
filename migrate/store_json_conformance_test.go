package migrate_test

import (
	"testing"

	"github.com/spoofcanary/dns-entree/migrate"
	"github.com/spoofcanary/dns-entree/migrate/storetest"
)

// TestJSONStoreConformance runs the full MigrationStore contract suite
// against the default JSON-files backend. The JSONStore constructor and
// implementation land in Plan 07-02; until that commit merges this test
// will fail to compile or run, which is the intended coordination signal.
func TestJSONStoreConformance(t *testing.T) {
	storetest.RunConformance(t, func(t *testing.T) migrate.MigrationStore {
		s, err := migrate.NewJSONStore(t.TempDir())
		if err != nil {
			t.Fatalf("NewJSONStore: %v", err)
		}
		return s
	})
}
