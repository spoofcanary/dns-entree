package entree

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// PushStatus enumerates push outcomes.
type PushStatus string

const (
	StatusCreated           PushStatus = "created"
	StatusUpdated           PushStatus = "updated"
	StatusAlreadyConfigured PushStatus = "already_configured"
	StatusFailed            PushStatus = "failed"
)

// PushResult describes the outcome of a push operation.
type PushResult struct {
	Status        PushStatus
	RecordName    string
	RecordValue   string
	PreviousValue string
	Verified      bool
	VerifyError   error
}

// PushService wraps any Provider with idempotent push + post-push verification.
type PushService struct {
	provider Provider
	logger   *slog.Logger
}

// NewPushService returns a PushService backed by the given provider.
func NewPushService(provider Provider) *PushService {
	return &PushService{provider: provider, logger: slog.Default()}
}

// Provider returns the underlying Provider. Used by higher-level orchestrators
// (e.g. template.ApplyTemplate) that need raw GetRecords/DeleteRecord access
// for conflict resolution.
func (s *PushService) Provider() Provider { return s.provider }

// verifyFunc is a package-level seam so tests can stub DNS verification.
var verifyFunc = Verify

// SetVerifyFuncForTest replaces the package-level verify seam and returns a
// restore function. Intended for use by external test packages (e.g.
// template) that need offline push verification.
func SetVerifyFuncForTest(fn func(ctx context.Context, domain string, opts VerifyOpts) (VerifyResult, error)) func() {
	prev := verifyFunc
	verifyFunc = fn
	return func() { verifyFunc = prev }
}

func normalizeHost(s string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
}

func findRecord(records []Record, name string) *Record {
	for i := range records {
		if records[i].Name == name {
			return &records[i]
		}
	}
	return nil
}

func (s *PushService) runVerify(ctx context.Context, domain string, opts VerifyOpts, result *PushResult) {
	vr, err := verifyFunc(ctx, domain, opts)
	if err != nil {
		result.VerifyError = err
		return
	}
	result.Verified = vr.Verified
	if !vr.Verified {
		result.VerifyError = errors.New("post-push verification did not observe record")
	}
}

// PushGenericRecord upserts an A/AAAA/MX/NS/SRV record idempotently with
// post-push verification. TXT and CNAME callers must use the typed methods
// (PushTXTRecord/PushCNAMERecord) which carry richer comparison semantics.
func (s *PushService) PushGenericRecord(ctx context.Context, domain string, record Record) (*PushResult, error) {
	switch record.Type {
	case "A", "AAAA", "MX", "NS", "SRV":
	case "TXT", "CNAME":
		return &PushResult{Status: StatusFailed, RecordName: record.Name, RecordValue: record.Content},
			fmt.Errorf("use PushTXTRecord/PushCNAMERecord for %s records", record.Type)
	default:
		return &PushResult{Status: StatusFailed, RecordName: record.Name, RecordValue: record.Content},
			fmt.Errorf("unsupported record type: %s", record.Type)
	}

	log := s.logger.With("domain", domain, "name", record.Name, "type", record.Type)
	result := &PushResult{RecordName: record.Name, RecordValue: record.Content}

	records, err := s.provider.GetRecords(ctx, domain, record.Type)
	if err != nil {
		result.Status = StatusFailed
		log.Error("get records failed", "error", err)
		return result, fmt.Errorf("get records: %w", err)
	}

	existing := findRecord(records, record.Name)
	if existing != nil && existing.Content == record.Content &&
		(record.Type != "MX" || existing.Priority == record.Priority) {
		result.Status = StatusAlreadyConfigured
		log.Info("already configured")
		return result, nil
	}
	if existing != nil {
		result.PreviousValue = existing.Content
	}

	if record.TTL == 0 {
		record.TTL = 300
	}

	if err := s.provider.SetRecord(ctx, domain, record); err != nil {
		result.Status = StatusFailed
		log.Error("set record failed", "error", err)
		return result, fmt.Errorf("set record: %w", err)
	}

	if existing != nil {
		result.Status = StatusUpdated
	} else {
		result.Status = StatusCreated
	}

	fingerprint := record.Content
	if len(fingerprint) > 32 {
		fingerprint = fingerprint[:32]
	}
	s.runVerify(ctx, domain, VerifyOpts{RecordType: record.Type, Name: record.Name, Contains: fingerprint}, result)
	log.Info("pushed", "status", result.Status, "verified", result.Verified)
	return result, nil
}

// PushTXTRecord upserts a TXT record at name with the given content.
func (s *PushService) PushTXTRecord(ctx context.Context, domain, name, content string) (*PushResult, error) {
	log := s.logger.With("domain", domain, "name", name, "type", "TXT")
	result := &PushResult{RecordName: name, RecordValue: content}

	records, err := s.provider.GetRecords(ctx, domain, "TXT")
	if err != nil {
		result.Status = StatusFailed
		log.Error("get records failed", "error", err)
		return result, fmt.Errorf("get records: %w", err)
	}

	existing := findRecord(records, name)
	if existing != nil && existing.Content == content {
		result.Status = StatusAlreadyConfigured
		log.Info("already configured")
		return result, nil
	}
	if existing != nil {
		result.PreviousValue = existing.Content
	}

	if err := s.provider.SetRecord(ctx, domain, Record{Type: "TXT", Name: name, Content: content, TTL: 300}); err != nil {
		result.Status = StatusFailed
		log.Error("set record failed", "error", err)
		return result, fmt.Errorf("set record: %w", err)
	}

	if existing != nil {
		result.Status = StatusUpdated
	} else {
		result.Status = StatusCreated
	}

	fingerprint := content
	if len(fingerprint) > 32 {
		fingerprint = fingerprint[:32]
	}
	s.runVerify(ctx, domain, VerifyOpts{RecordType: "TXT", Name: name, Contains: fingerprint}, result)
	log.Info("pushed", "status", result.Status, "verified", result.Verified)
	return result, nil
}

// PushSPFRecord merges includes into any existing SPF record at the apex.
func (s *PushService) PushSPFRecord(ctx context.Context, domain string, includes []string) (*PushResult, error) {
	log := s.logger.With("domain", domain, "type", "SPF")
	result := &PushResult{RecordName: domain}

	records, err := s.provider.GetRecords(ctx, domain, "TXT")
	if err != nil {
		result.Status = StatusFailed
		log.Error("get records failed", "error", err)
		return result, fmt.Errorf("get records: %w", err)
	}

	var currentSPF string
	for _, r := range records {
		if r.Name == domain && strings.HasPrefix(strings.ToLower(r.Content), "v=spf1") {
			currentSPF = r.Content
			break
		}
	}

	merge, _ := MergeSPF(currentSPF, includes)
	result.RecordValue = merge.Value
	if currentSPF != "" {
		result.PreviousValue = currentSPF
	}

	if !merge.Changed {
		result.Status = StatusAlreadyConfigured
		log.Info("already configured")
		return result, nil
	}

	if err := s.provider.SetRecord(ctx, domain, Record{Type: "TXT", Name: domain, Content: merge.Value, TTL: 300}); err != nil {
		result.Status = StatusFailed
		log.Error("set record failed", "error", err)
		return result, fmt.Errorf("set record: %w", err)
	}

	if currentSPF == "" {
		result.Status = StatusCreated
	} else {
		result.Status = StatusUpdated
	}

	s.runVerify(ctx, domain, VerifyOpts{RecordType: "TXT", Name: domain, Contains: "v=spf1"}, result)
	log.Info("pushed", "status", result.Status, "verified", result.Verified)
	return result, nil
}

// PushCNAMERecord upserts a CNAME at name pointing at target. Trailing dots
// and case are ignored when comparing existing records.
func (s *PushService) PushCNAMERecord(ctx context.Context, domain, name, target string) (*PushResult, error) {
	log := s.logger.With("domain", domain, "name", name, "type", "CNAME")
	result := &PushResult{RecordName: name, RecordValue: target}

	records, err := s.provider.GetRecords(ctx, domain, "CNAME")
	if err != nil {
		result.Status = StatusFailed
		log.Error("get records failed", "error", err)
		return result, fmt.Errorf("get records: %w", err)
	}

	existing := findRecord(records, name)
	if existing != nil && normalizeHost(existing.Content) == normalizeHost(target) {
		result.Status = StatusAlreadyConfigured
		log.Info("already configured")
		return result, nil
	}
	if existing != nil {
		result.PreviousValue = existing.Content
	}

	if err := s.provider.SetRecord(ctx, domain, Record{Type: "CNAME", Name: name, Content: target, TTL: 300}); err != nil {
		result.Status = StatusFailed
		log.Error("set record failed", "error", err)
		return result, fmt.Errorf("set record: %w", err)
	}

	if existing != nil {
		result.Status = StatusUpdated
	} else {
		result.Status = StatusCreated
	}

	s.runVerify(ctx, domain, VerifyOpts{RecordType: "CNAME", Name: name, Contains: normalizeHost(target)}, result)
	log.Info("pushed", "status", result.Status, "verified", result.Verified)
	return result, nil
}
