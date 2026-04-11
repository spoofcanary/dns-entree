// Package storetest provides a reusable conformance suite for any
// MigrationStore implementation. Backends import this package in their
// _test.go and call RunConformance with a factory that returns a fresh,
// empty store per test case.
//
// Contract guarantees exercised by RunConformance:
//   - Create rejects duplicate IDs
//   - Get on unknown ID returns ErrNotFound
//   - Update enforces optimistic locking: the input's Version must equal
//     the stored Version + 1, otherwise ErrVersionMismatch
//   - Update on unknown ID returns ErrNotFound
//   - Delete is idempotent (unknown ID returns nil)
//   - List with zero filter returns every row
//   - List filters by TenantID and Status independently
//   - List honors Limit/Offset pagination
//   - SweepExpired deletes rows where ExpiresAt is strictly before the cutoff
//     and returns the number of rows removed
//   - All sentinel errors are checked with errors.Is
package storetest
