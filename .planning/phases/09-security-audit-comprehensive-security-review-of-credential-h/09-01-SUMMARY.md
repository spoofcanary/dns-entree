---
phase: 09-security-audit
plan: 01
subsystem: api
tags: [validation, dns, rfc1035, security, input-validation]

# Dependency graph
requires: []
provides:
  - "ValidateDNSName and ValidateRecordValue exported functions in package entree"
  - "Input validation at all API handler entry points"
  - "Domain validation in domainconnect.Discover"
affects: [api, domainconnect, template]

# Tech tracking
tech-stack:
  added: []
  patterns: ["centralized validation at package root", "per-handler validation before network I/O"]

key-files:
  created:
    - validate.go
    - validate_test.go
  modified:
    - api/handlers_core.go
    - api/handlers_core_test.go
    - api/handlers_dc.go
    - api/handlers_migrate.go
    - api/handlers_migrate_stateful.go
    - domainconnect/discovery.go

key-decisions:
  - "Duplicate minimal domain validation in domainconnect package to avoid circular import"
  - "Unknown record types pass through ValidateRecordValue (lenient parsing per D-04)"
  - "Underscore and trailing dot tolerance in ValidateDNSName (D-05 edge cases)"

patterns-established:
  - "Validate input at handler entry before any network I/O or provider calls"
  - "Return BAD_REQUEST code for all validation failures with descriptive messages"

requirements-completed: []

# Metrics
duration: 5min
completed: 2026-04-11
---

# Phase 9 Plan 1: Input Validation Summary

**RFC 1035 DNS name validation and type-specific record value validation wired into all API handlers**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-11T19:17:43Z
- **Completed:** 2026-04-11T19:22:23Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Created ValidateDNSName with RFC 1035 compliance plus underscore/trailing dot/wildcard tolerance
- Created ValidateRecordValue with type-specific checks for A, AAAA, CNAME, MX, TXT, NS, SRV
- Wired validation into all 9 API handler entry points (detect, verify, apply, apply-template, dc-discover, dc-apply-url, migrate, zone-export, migrate-preview)
- Added domain validation to domainconnect.Discover with SSRF-safe local implementation
- 50+ test cases covering valid input, edge cases, and malformed input

## Task Commits

Each task was committed atomically:

1. **Task 1: Create shared validate.go (TDD RED)** - `c7ded44` (test)
2. **Task 1: Create shared validate.go (TDD GREEN)** - `b3f6f0c` (feat)
3. **Task 2: Wire validation into API handlers** - `f58c962` (feat)

## Files Created/Modified
- `validate.go` - ValidateDNSName and ValidateRecordValue exported functions
- `validate_test.go` - 50+ table-driven test cases for both functions
- `api/handlers_core.go` - Validation in handleDetect, handleVerify, handleApply, handleApplyTemplate
- `api/handlers_core_test.go` - 5 new validation tests for invalid domain/record rejection
- `api/handlers_dc.go` - Validation in handleDCDiscover and handleDCApplyURL
- `api/handlers_migrate.go` - Validation in handleMigrate and handleZoneExport
- `api/handlers_migrate_stateful.go` - Validation in handleMigratePreview
- `domainconnect/discovery.go` - Local validateDomain to avoid circular import

## Decisions Made
- Used a local validateDomain function in domainconnect package rather than importing entree to avoid circular dependency. The local version covers the same essential checks (empty, length, consecutive dots, forbidden chars, TLD presence).
- Unknown record types pass through ValidateRecordValue with no error, preserving forward compatibility.
- Record name validation in handleApply only runs when Name is non-empty (empty name means apex).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Test case for "exactly 253 chars total" had an off-by-one (computed 254 chars) - fixed in GREEN phase.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All API entry points now validate DNS names and record values before network calls
- Validation functions are available for any future handlers or library consumers
- T-09-01 through T-09-03 threat mitigations implemented

## Self-Check: PASSED

All 8 files exist. All 3 commits found in git log.

---
*Phase: 09-security-audit*
*Completed: 2026-04-11*
