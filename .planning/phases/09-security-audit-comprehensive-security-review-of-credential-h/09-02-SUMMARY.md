---
phase: 09-security-audit
plan: 02
subsystem: testing
tags: [fuzz-testing, gosec, staticcheck, aes-gcm, ssrf, credential-redaction, security]

# Dependency graph
requires:
  - phase: 09-security-audit
    provides: credential handling audit context and threat model
provides:
  - Security test suite covering credential redaction, error scrubbing, timing-safe comparison
  - Three fuzz targets for SPF merge, Domain Connect discovery, DNS name validation
  - ValidateDNSName function (RFC 1035/1123 compliant)
  - AES-GCM property tests (round-trip, tamper detection, wrong key, nonce uniqueness)
affects: [security-audit, testing]

# Tech tracking
tech-stack:
  added: [gosec, staticcheck]
  patterns: [go-native-fuzzing, code-property-tests, defense-in-depth-testing]

key-files:
  created:
    - api/security_test.go
    - security_test.go
    - migrate/security_test.go
    - spf_fuzz_test.go
    - domainconnect/discovery_fuzz_test.go
    - validate_fuzz_test.go
    - validate.go
  modified: []

key-decisions:
  - "AES-GCM tests placed in migrate/ package to avoid import cycle (entree -> migrate -> entree)"
  - "Created ValidateDNSName function in validate.go since Plan 01 had not created it yet"
  - "Pre-existing gosec/staticcheck findings in unmodified files left as-is (out of scope)"

patterns-established:
  - "Code-property tests: grep source files to assert security invariants (e.g. ConstantTimeCompare usage)"
  - "Fuzz test seed corpus: include valid, empty, malicious, and boundary inputs"

requirements-completed: []

# Metrics
duration: 6min
completed: 2026-04-11
---

# Phase 9 Plan 2: Security Tests & Fuzzing Summary

**Security test suite with 11 test functions covering credential redaction, SSRF, AES-GCM crypto properties, and 3 fuzz targets for parser crash resistance**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-11T19:17:53Z
- **Completed:** 2026-04-11T19:24:15Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Security test suite covering credential redaction (exhaustive header scrubbing), error detail stripping, and timing-safe token comparison verification
- AES-GCM property tests: round-trip (0B to 1MB), tamper detection, wrong-key rejection, nonce uniqueness
- Three Go native fuzz targets exercising MergeSPF, DiscoverDomainConnect, and ValidateDNSName without panics
- gosec and staticcheck audit completed; all findings are pre-existing in unmodified files

## Task Commits

Each task was committed atomically:

1. **Task 1: Create security-focused test suite** - `a932d2e` (test)
2. **Task 2: Add fuzz targets and run gosec/staticcheck** - `579bfed` (feat)

## Files Created/Modified
- `api/security_test.go` - Credential redaction exhaustive test, scrubDetails tests, timing-safe comparison test (5 test functions)
- `security_test.go` - Credential struct leak detection, MergeSPF edge-case security tests (2 test functions)
- `migrate/security_test.go` - AES-GCM round-trip, tamper detection, wrong key, nonce uniqueness (4 test functions)
- `spf_fuzz_test.go` - FuzzMergeSPF fuzz target with seed corpus
- `domainconnect/discovery_fuzz_test.go` - FuzzDiscoverDomainConnect fuzz target with SSRF seeds
- `validate_fuzz_test.go` - FuzzValidateDNSName fuzz target
- `validate.go` - ValidateDNSName RFC 1035/1123 DNS name validation function

## Decisions Made
- AES-GCM tests placed in `migrate/` package (not root) to avoid import cycle: `entree -> migrate -> entree`
- Created `ValidateDNSName` in `validate.go` since Plan 01 had not created it; needed for fuzz target to compile
- gosec HIGH findings (G115 integer overflow in store_json_unix.go, G704 SSRF in godaddy.go, G122 symlink in sync.go) are all pre-existing in unmodified files -- documented but not fixed per scope boundary rule
- staticcheck findings (2 unused functions in cmd/entree/) also pre-existing, out of scope

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Created ValidateDNSName function**
- **Found during:** Task 2 (fuzz target creation)
- **Issue:** Plan specified FuzzValidateDNSName but ValidateDNSName does not exist (Plan 01 was noted as potential source)
- **Fix:** Created validate.go with RFC 1035/1123 compliant ValidateDNSName function
- **Files modified:** validate.go
- **Verification:** FuzzValidateDNSName runs 5+ seconds without panics, go build/test pass
- **Committed in:** 579bfed (Task 2 commit)

**2. [Rule 1 - Bug] Fixed invalid HTTP header value in test**
- **Found during:** Task 1 (security test suite)
- **Issue:** Test used literal newline in header value (`key\nnewline`) which is invalid per HTTP spec
- **Fix:** Replaced with special characters valid in HTTP headers (`key-with-special=chars&more`)
- **Files modified:** api/security_test.go
- **Verification:** Test passes
- **Committed in:** a932d2e (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both auto-fixes necessary for correctness. No scope creep.

## gosec/staticcheck Audit Results

### gosec Findings (all pre-existing, out of scope)

| Severity | Rule | File | Description | Disposition |
|----------|------|------|-------------|-------------|
| HIGH | G115 | migrate/store_json_unix.go | uintptr->int conversion | False positive on 64-bit; pre-existing |
| HIGH | G704 | providers/godaddy/godaddy.go | SSRF via taint analysis | URL from known API base; pre-existing |
| HIGH | G122 | template/sync.go | symlink TOCTOU in WalkDir | Pre-existing template sync code |
| MEDIUM | G304 | multiple files | File inclusion via variable | Expected for file I/O utilities |
| MEDIUM | G117 | 2 files | Marshaled secret patterns | Intentional (access tokens in store) |
| LOW | G104 | 5 files | Unhandled errors | Pre-existing |

### staticcheck Findings (all pre-existing, out of scope)

| File | Finding |
|------|---------|
| cmd/entree/cli_template_test.go | unused func runCmd |
| cmd/entree/root.go | unused func loggerFromCtx |

## Issues Encountered
- Import cycle prevented placing AES-GCM tests in root package `entree` (which imports `migrate`, which imports `entree`). Resolved by placing crypto tests in `migrate/security_test.go`.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Security test foundation complete for future regression testing
- Fuzz targets can be run in CI with `-fuzztime` flag for continuous fuzzing
- ValidateDNSName function ready for use by other packages

## Self-Check: PASSED

All 7 created files verified present. Both task commits (a932d2e, 579bfed) verified in git log.

---
*Phase: 09-security-audit*
*Completed: 2026-04-11*
