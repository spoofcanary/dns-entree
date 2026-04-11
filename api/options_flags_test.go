package api

import (
	"flag"
	"testing"
	"time"
)

func parse(t *testing.T, args []string, env map[string]string) *Options {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	b := BindFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	if err := LoadFromEnv(fs, b, func(k string) string { return env[k] }); err != nil {
		t.Fatal(err)
	}
	return b.Opts
}

func TestBindFlags_Defaults(t *testing.T) {
	o := parse(t, nil, nil)
	if o.Listen != ":8080" {
		t.Errorf("listen=%q", o.Listen)
	}
	if o.LogLevel != "info" {
		t.Errorf("log-level=%q", o.LogLevel)
	}
	if o.LogFormat != "json" {
		t.Errorf("log-format=%q", o.LogFormat)
	}
	if o.RequestTimeout != 15*time.Minute {
		t.Errorf("timeout=%v", o.RequestTimeout)
	}
}

func TestLoadFromEnv_AppliesWhenFlagUnset(t *testing.T) {
	o := parse(t, nil, map[string]string{
		EnvListen:           ":9090",
		EnvLogLevel:         "debug",
		EnvLogFormat:        "text",
		EnvRequestTimeout:   "5m",
		EnvCORSOrigin:       "https://a.example, https://b.example",
		EnvTemplateCacheDir: "/tmp/x",
	})
	if o.Listen != ":9090" {
		t.Errorf("listen=%q", o.Listen)
	}
	if o.LogLevel != "debug" {
		t.Errorf("log-level=%q", o.LogLevel)
	}
	if o.LogFormat != "text" {
		t.Errorf("log-format=%q", o.LogFormat)
	}
	if o.RequestTimeout != 5*time.Minute {
		t.Errorf("timeout=%v", o.RequestTimeout)
	}
	if o.TemplateCacheDir != "/tmp/x" {
		t.Errorf("tcd=%q", o.TemplateCacheDir)
	}
	if len(o.CORSOrigins) != 2 || o.CORSOrigins[0] != "https://a.example" || o.CORSOrigins[1] != "https://b.example" {
		t.Errorf("cors=%v", o.CORSOrigins)
	}
}

func TestLoadFromEnv_FlagBeatsEnv(t *testing.T) {
	o := parse(t, []string{"--listen", ":7777", "--log-level", "warn", "--cors-origin", "https://flag.example"}, map[string]string{
		EnvListen:     ":9090",
		EnvLogLevel:   "debug",
		EnvCORSOrigin: "https://env.example",
	})
	if o.Listen != ":7777" {
		t.Errorf("listen=%q", o.Listen)
	}
	if o.LogLevel != "warn" {
		t.Errorf("log-level=%q", o.LogLevel)
	}
	if len(o.CORSOrigins) != 1 || o.CORSOrigins[0] != "https://flag.example" {
		t.Errorf("cors=%v", o.CORSOrigins)
	}
}

func TestLoadFromEnv_RepeatableCORSFlag(t *testing.T) {
	o := parse(t, []string{"--cors-origin", "https://a", "--cors-origin", "https://b"}, nil)
	if len(o.CORSOrigins) != 2 {
		t.Fatalf("cors=%v", o.CORSOrigins)
	}
	if o.CORSOrigins[0] != "https://a" || o.CORSOrigins[1] != "https://b" {
		t.Errorf("cors=%v", o.CORSOrigins)
	}
}

func TestLoadFromEnv_BadDuration(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	b := BindFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	err := LoadFromEnv(fs, b, func(k string) string {
		if k == EnvRequestTimeout {
			return "not-a-duration"
		}
		return ""
	})
	if err == nil {
		t.Fatal("expected error on bad duration")
	}
}
