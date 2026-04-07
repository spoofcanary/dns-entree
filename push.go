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

// verifyFunc is a package-level seam so tests can stub DNS verification.
var verifyFunc = Verify

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
