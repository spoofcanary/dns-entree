package template

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
)

const (
	defaultRepoURL = "https://github.com/Domain-Connect/Templates.git"
	defaultTTL     = 24 * time.Hour
	sentinelFile   = ".dns-entree-synced"
)

// SyncOption configures sync/load behavior.
type SyncOption func(*syncConfig)

type syncConfig struct {
	cacheDir string
	ttl      time.Duration
	repoURL  string
	logger   *slog.Logger
}

// WithCacheDir overrides the default XDG cache location.
func WithCacheDir(dir string) SyncOption {
	return func(c *syncConfig) { c.cacheDir = dir }
}

// WithCacheTTL controls auto-refresh staleness threshold.
// 0 = always refresh; negative = never auto-refresh.
func WithCacheTTL(d time.Duration) SyncOption {
	return func(c *syncConfig) { c.ttl = d }
}

// WithRepoURL overrides the upstream repo (mainly for tests, e.g. file://).
func WithRepoURL(url string) SyncOption {
	return func(c *syncConfig) { c.repoURL = url }
}

// WithSyncLogger attaches a structured logger to sync operations.
func WithSyncLogger(l *slog.Logger) SyncOption {
	return func(c *syncConfig) { c.logger = l }
}

func resolveConfig(opts ...SyncOption) (*syncConfig, error) {
	cfg := &syncConfig{
		ttl:     defaultTTL,
		repoURL: defaultRepoURL,
	}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.cacheDir == "" {
		d, err := defaultCacheDir()
		if err != nil {
			return nil, fmt.Errorf("resolve cache dir: %w", err)
		}
		cfg.cacheDir = d
	}
	if cfg.logger == nil {
		cfg.logger = slog.New(slog.NewTextHandler(nopWriter{}, nil))
	}
	return cfg, nil
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

// defaultCacheDir resolves $XDG_CACHE_HOME/dns-entree/templates, falling back
// to os.UserCacheDir. Manual XDG check is required because os.UserCacheDir on
// macOS ignores XDG_CACHE_HOME.
func defaultCacheDir() (string, error) {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, "dns-entree", "templates"), nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "dns-entree", "templates"), nil
}

// SyncTemplates clones or fast-forwards the Domain Connect templates repo
// into the cache directory.
func SyncTemplates(ctx context.Context, opts ...SyncOption) error {
	cfg, err := resolveConfig(opts...)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.cacheDir), 0o755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	gitDir := filepath.Join(cfg.cacheDir, ".git")
	if _, err := os.Stat(gitDir); errors.Is(err, fs.ErrNotExist) {
		cfg.logger.Info("cloning template repo", "url", cfg.repoURL, "dir", cfg.cacheDir)
		if err := os.MkdirAll(cfg.cacheDir, 0o755); err != nil {
			return fmt.Errorf("mkdir cache: %w", err)
		}
		_, err := git.PlainCloneContext(ctx, cfg.cacheDir, false, &git.CloneOptions{
			URL:          cfg.repoURL,
			Depth:        1,
			SingleBranch: true,
		})
		if err != nil {
			return fmt.Errorf("clone: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("stat .git: %w", err)
	} else {
		repo, err := git.PlainOpen(cfg.cacheDir)
		if err != nil {
			return fmt.Errorf("open repo: %w", err)
		}
		w, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("worktree: %w", err)
		}
		err = w.PullContext(ctx, &git.PullOptions{SingleBranch: true})
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return fmt.Errorf("pull: %w", err)
		}
		cfg.logger.Info("synced template repo", "dir", cfg.cacheDir)
	}

	// Touch sentinel for TTL tracking.
	sentPath := filepath.Join(cfg.cacheDir, sentinelFile)
	if f, err := os.Create(sentPath); err == nil {
		_ = f.Close()
	}
	now := time.Now()
	_ = os.Chtimes(sentPath, now, now)
	return nil
}

// TemplateSummary describes one cached template.
type TemplateSummary struct {
	ProviderID   string `json:"providerId"`
	ServiceID    string `json:"serviceId"`
	ProviderName string `json:"providerName"`
	ServiceName  string `json:"serviceName"`
	Path         string `json:"path"`
}

// LoadTemplate loads a single template by provider/service ID, auto-syncing
// if the cache is stale per the configured TTL.
func LoadTemplate(ctx context.Context, providerID, serviceID string, opts ...SyncOption) (*Template, error) {
	if err := validateID(providerID); err != nil {
		return nil, fmt.Errorf("providerID: %w", err)
	}
	if err := validateID(serviceID); err != nil {
		return nil, fmt.Errorf("serviceID: %w", err)
	}
	cfg, err := resolveConfig(opts...)
	if err != nil {
		return nil, err
	}
	if cacheStale(cfg.cacheDir, cfg.ttl) {
		if err := SyncTemplates(ctx, opts...); err != nil {
			return nil, fmt.Errorf("auto-sync: %w", err)
		}
	}
	path := filepath.Join(cfg.cacheDir, providerID, providerID+"."+serviceID+".json")
	return loadTemplateFile(path)
}

// ListTemplates walks the cache and returns one summary per *.json template.
func ListTemplates(opts ...SyncOption) ([]TemplateSummary, error) {
	cfg, err := resolveConfig(opts...)
	if err != nil {
		return nil, err
	}
	var out []TemplateSummary
	err = filepath.WalkDir(cfg.cacheDir, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			if errors.Is(werr, fs.ErrNotExist) {
				return nil
			}
			return werr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		var head struct {
			ProviderID   string `json:"providerId"`
			ServiceID    string `json:"serviceId"`
			ProviderName string `json:"providerName"`
			ServiceName  string `json:"serviceName"`
		}
		if err := json.NewDecoder(f).Decode(&head); err != nil {
			return nil
		}
		if head.ProviderID == "" && head.ServiceID == "" {
			return nil
		}
		out = append(out, TemplateSummary{
			ProviderID:   head.ProviderID,
			ServiceID:    head.ServiceID,
			ProviderName: head.ProviderName,
			ServiceName:  head.ServiceName,
			Path:         path,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProviderID != out[j].ProviderID {
			return out[i].ProviderID < out[j].ProviderID
		}
		return out[i].ServiceID < out[j].ServiceID
	})
	return out, nil
}

func cacheStale(cacheDir string, ttl time.Duration) bool {
	if ttl < 0 {
		return false
	}
	if ttl == 0 {
		return true
	}
	info, err := os.Stat(filepath.Join(cacheDir, sentinelFile))
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > ttl
}

func validateID(id string) error {
	if id == "" {
		return errors.New("empty")
	}
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return errors.New("invalid characters")
	}
	return nil
}

// loadTemplateFile is a thin wrapper that defers to LoadTemplateFile from
// template.go (provided by plan 04-02). Declared here as a var so tests in
// this file can stub it before 04-02 lands; once 04-02's template.go is
// merged, this var simply forwards to it.
var loadTemplateFile = func(path string) (*Template, error) {
	return LoadTemplateFile(path)
}
