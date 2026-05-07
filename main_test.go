package main

import "testing"

func TestResolveServerPortUsesFlagValue(t *testing.T) {
	t.Setenv(serverPortEnvVar, "9090")

	got, err := resolveServerPort(true, 7070)
	if err != nil {
		t.Fatalf("resolveServerPort() error = %v", err)
	}

	if got != 7070 {
		t.Fatalf("resolveServerPort() = %d, want %d", got, 7070)
	}
}

func TestResolveServerPortUsesEnvWhenFlagMissing(t *testing.T) {
	t.Setenv(serverPortEnvVar, "9091")

	got, err := resolveServerPort(false, defaultServerPort)
	if err != nil {
		t.Fatalf("resolveServerPort() error = %v", err)
	}

	if got != 9091 {
		t.Fatalf("resolveServerPort() = %d, want %d", got, 9091)
	}
}