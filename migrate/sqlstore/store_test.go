//go:build sqlite

package sqlstore_test

import (
	"path/filepath"
	"testing"

	"github.com/spoofcanary/dns-entree/migrate"
	"github.com/spoofcanary/dns-entree/migrate/sqlstore"
	"github.com/spoofcanary/dns-entree/migrate/storetest"
)

func TestSQLiteStoreConformance(t *testing.T) {
	storetest.RunConformance(t, func(t *testing.T) migrate.MigrationStore {
		p := filepath.Join(t.TempDir(), "test.db")
		s, err := sqlstore.New(p)
		if err != nil {
			t.Fatalf("NewSQLiteStore: %v", err)
		}
		return s
	})
}
