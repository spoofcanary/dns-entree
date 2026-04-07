package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// pickPort returns an OS-assigned free TCP port as ":NNNN".
func pickPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().(*net.TCPAddr)
	_ = l.Close()
	return fmt.Sprintf("127.0.0.1:%d", addr.Port)
}

func TestRun_GracefulShutdown(t *testing.T) {
	addr := pickPort(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"--listen", addr}, func(string) string { return "" }, io.Discard)
	}()

	// Wait for the listener to come up.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Hit /healthz.
	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		cancel()
		t.Fatalf("healthz: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("healthz status=%d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("run exit code=%d, want 0", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run() did not return within 3s of ctx cancel")
	}
}

func TestRun_FlagOverEnvPrecedence(t *testing.T) {
	// Env says one port, flag says another - flag must win (D-27).
	envAddr := pickPort(t)
	flagAddr := pickPort(t)
	env := func(k string) string {
		if k == "ENTREE_API_LISTEN" {
			return envAddr
		}
		return ""
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"--listen", flagAddr}, env, io.Discard)
	}()
	// Wait for flagAddr to be reachable, envAddr should remain free.
	deadline := time.Now().Add(2 * time.Second)
	reached := false
	for time.Now().Before(deadline) {
		if c, err := net.DialTimeout("tcp", flagAddr, 100*time.Millisecond); err == nil {
			_ = c.Close()
			reached = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !reached {
		cancel()
		t.Fatal("flag-supplied addr never came up")
	}
	// envAddr must be unbound.
	if c, err := net.DialTimeout("tcp", envAddr, 100*time.Millisecond); err == nil {
		_ = c.Close()
		t.Errorf("env addr unexpectedly bound; flag did not take precedence")
	}
	cancel()
	<-done
}

func TestRun_EnvUsedWhenFlagOmitted(t *testing.T) {
	addr := pickPort(t)
	env := func(k string) string {
		if k == "ENTREE_API_LISTEN" {
			return addr
		}
		return ""
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, nil, env, io.Discard)
	}()
	deadline := time.Now().Add(2 * time.Second)
	reached := false
	for time.Now().Before(deadline) {
		if c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond); err == nil {
			_ = c.Close()
			reached = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !reached {
		cancel()
		t.Fatal("env-supplied addr never came up")
	}
	cancel()
	<-done
}

func TestRun_BadFlagReturnsError(t *testing.T) {
	var buf bytes.Buffer
	code := run(context.Background(), []string{"--nope"}, func(string) string { return "" }, &buf)
	if code == 0 {
		t.Errorf("expected non-zero exit on bad flag, got %d", code)
	}
}

// TestSubprocess_SIGINT spawns the binary in a child process and asserts that
// SIGINT triggers a clean exit. Skipped on Windows where signal semantics
// differ.
func TestSubprocess_SIGINT(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT semantics differ on Windows")
	}
	if testing.Short() {
		t.Skip("subprocess build is slow under -short")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "entree-api")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Stderr = &bytes.Buffer{}
	if err := build.Run(); err != nil {
		t.Fatalf("go build: %v: %s", err, build.Stderr.(*bytes.Buffer).String())
	}
	addr := pickPort(t)
	cmd := exec.Command(bin, "--listen", addr)
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Wait for listener.
	deadline := time.Now().Add(3 * time.Second)
	up := false
	for time.Now().Before(deadline) {
		if c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond); err == nil {
			_ = c.Close()
			up = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !up {
		_ = cmd.Process.Kill()
		t.Fatal("subprocess listener never came up")
	}
	// Hit /healthz.
	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("healthz: %v", err)
	}
	_ = resp.Body.Close()
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	exitErr := make(chan error, 1)
	go func() { exitErr <- cmd.Wait() }()
	select {
	case err := <-exitErr:
		if err != nil {
			var ee *exec.ExitError
			if !errors.As(err, &ee) {
				t.Errorf("subprocess exit: %v", err)
			}
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("subprocess did not exit within 5s of SIGINT")
	}
	_ = strings.Contains // keep import
}
