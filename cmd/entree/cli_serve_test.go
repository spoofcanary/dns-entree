package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func pickPortServe(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().(*net.TCPAddr)
	_ = l.Close()
	return fmt.Sprintf("127.0.0.1:%d", addr.Port)
}

// TestServeSubcommand_SIGINT builds the entree CLI and runs `entree serve`,
// asserts /healthz responds, then sends SIGINT and verifies clean exit.
func TestServeSubcommand_SIGINT(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT semantics differ on Windows")
	}
	if testing.Short() {
		t.Skip("subprocess build is slow under -short")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "entree")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Stderr = &bytes.Buffer{}
	if err := build.Run(); err != nil {
		t.Fatalf("go build: %v: %s", err, build.Stderr.(*bytes.Buffer).String())
	}
	addr := pickPortServe(t)
	cmd := exec.Command(bin, "serve", "--listen", addr)
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
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
		t.Fatal("entree serve listener never came up")
	}
	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("healthz: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("healthz status=%d", resp.StatusCode)
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
				t.Errorf("entree serve exit: %v", err)
			}
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("entree serve did not exit within 5s of SIGINT")
	}
}
