package server

import (
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
