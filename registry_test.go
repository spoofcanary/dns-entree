package entree

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
)

type mockProvider struct {
	name           string
	slug           string
	setRecordCalls int
	failOnCall     int // 1-indexed; 0 = never fail
	setRecordErr   error
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Slug() string { return m.slug }
func (m *mockProvider) Verify(ctx context.Context) ([]Zone, error) {
	return nil, nil
}
func (m *mockProvider) GetRecords(ctx context.Context, domain, recordType string) ([]Record, error) {
	return nil, nil
}
func (m *mockProvider) SetRecord(ctx context.Context, domain string, record Record) error {
	m.setRecordCalls++
	if m.failOnCall > 0 && m.setRecordCalls == m.failOnCall {
		return m.setRecordErr
	}
	return nil
}
func (m *mockProvider) DeleteRecord(ctx context.Context, domain, recordID string) error {
	return nil
}
func (m *mockProvider) ApplyRecords(ctx context.Context, domain string, records []Record) error {
	return DefaultApplyRecords(m, ctx, domain, records)
}

// resetRegistry clears the global registry between tests.
func resetRegistry() {
	registryMu.Lock()
	registry = map[string]ProviderFactory{}
	registryMu.Unlock()
}

func TestNewProvider_UnknownSlug(t *testing.T) {
	resetRegistry()
	_, err := NewProvider("nonexistent", Credentials{})
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected error containing 'unknown provider', got: %v", err)
	}
}

func TestRegisterAndNewProvider(t *testing.T) {
	resetRegistry()
	mock := &mockProvider{name: "Test", slug: "test"}
	RegisterProvider("test", func(creds Credentials) (Provider, error) {
		return mock, nil
	})
	p, err := NewProvider("test", Credentials{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Slug() != "test" {
		t.Errorf("expected slug 'test', got %q", p.Slug())
	}
	if p.Name() != "Test" {
		t.Errorf("expected name 'Test', got %q", p.Name())
	}
}

func TestRegisteredProviders(t *testing.T) {
	resetRegistry()
	RegisterProvider("alpha", func(creds Credentials) (Provider, error) {
		return &mockProvider{slug: "alpha"}, nil
	})
	RegisterProvider("beta", func(creds Credentials) (Provider, error) {
		return &mockProvider{slug: "beta"}, nil
	})
	got := RegisteredProviders()
	sort.Strings(got)
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Errorf("expected [alpha beta], got %v", got)
	}
}

func TestNewProvider_FactoryError(t *testing.T) {
	resetRegistry()
	wantErr := errors.New("bad creds")
	RegisterProvider("broken", func(creds Credentials) (Provider, error) {
		return nil, wantErr
	})
	_, err := NewProvider("broken", Credentials{})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected factory error to propagate, got: %v", err)
	}
}

func TestDefaultApplyRecords_Success(t *testing.T) {
	m := &mockProvider{}
	records := []Record{
		{Type: "TXT", Name: "a.example.com", Content: "v1"},
		{Type: "TXT", Name: "b.example.com", Content: "v2"},
		{Type: "TXT", Name: "c.example.com", Content: "v3"},
	}
	if err := DefaultApplyRecords(m, context.Background(), "example.com", records); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.setRecordCalls != 3 {
		t.Errorf("expected 3 SetRecord calls, got %d", m.setRecordCalls)
	}
}

func TestDefaultApplyRecords_ErrorStops(t *testing.T) {
	m := &mockProvider{failOnCall: 2, setRecordErr: fmt.Errorf("api down")}
	records := []Record{
		{Type: "TXT", Name: "a.example.com", Content: "v1"},
		{Type: "TXT", Name: "b.example.com", Content: "v2"},
		{Type: "TXT", Name: "c.example.com", Content: "v3"},
	}
	err := DefaultApplyRecords(m, context.Background(), "example.com", records)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "b.example.com") {
		t.Errorf("expected error to wrap record name, got: %v", err)
	}
	if m.setRecordCalls != 2 {
		t.Errorf("expected 2 SetRecord calls (stopped on failure), got %d", m.setRecordCalls)
	}
}
