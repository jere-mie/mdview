package server

import (
	"net"
	"path/filepath"
	"testing"
)

func TestResolvePath(t *testing.T) {
	root := filepath.Clean(`C:\docs`)

	resolved, err := resolvePath(root, `/notes/readme.md`)
	if err != nil {
		t.Fatalf("resolvePath() error = %v", err)
	}

	want := filepath.Join(root, "notes", "readme.md")
	if resolved != want {
		t.Fatalf("resolvePath() = %q, want %q", resolved, want)
	}
}

func TestResolvePathRejectsTraversal(t *testing.T) {
	root := filepath.Clean(`C:\docs`)

	if _, err := resolvePath(root, `/../secret.txt`); err == nil {
		t.Fatalf("expected traversal path to fail")
	}
}

func TestListenAddrDefaultsToLoopback(t *testing.T) {
	got := listenAddr("", 8080)
	want := "127.0.0.1:8080"
	if got != want {
		t.Fatalf("listenAddr() = %q, want %q", got, want)
	}
}

func TestListenWithFallbackSkipsBusyPort(t *testing.T) {
	for i := 0; i < 50; i++ {
		probe, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("net.Listen() = %v", err)
		}

		busyPort := probe.Addr().(*net.TCPAddr).Port
		_ = probe.Close()

		if busyPort >= 65535 {
			continue
		}

		nextListener, err := net.Listen("tcp", listenAddr("127.0.0.1", busyPort+1))
		if err != nil {
			continue
		}
		_ = nextListener.Close()

		busyListener, err := net.Listen("tcp", listenAddr("127.0.0.1", busyPort))
		if err != nil {
			continue
		}
		defer busyListener.Close()

		listener, actualPort, err := listenWithFallback("127.0.0.1", busyPort, 2)
		if err != nil {
			t.Fatalf("listenWithFallback() error = %v", err)
		}
		defer listener.Close()

		if actualPort != busyPort+1 {
			t.Fatalf("listenWithFallback() port = %d, want %d", actualPort, busyPort+1)
		}
		return
	}

	t.Skip("could not find a stable adjacent free port pair")
}
