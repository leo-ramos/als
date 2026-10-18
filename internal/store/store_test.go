package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leo-ramos/als/internal/config"
)

func setupTestStore(t *testing.T) (*Store, func()) {
	tmpDir, err := os.MkdirTemp("", "als-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		ConfigDir: filepath.Join(tmpDir, "config"),
		BinDir:    filepath.Join(tmpDir, "bin"),
	}
	st := New(cfg)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}
	return st, cleanup
}

func TestStore_SaveAndUpdateCommand(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	// 1. Save initial command
	cmd := Command{
		Cmd: "fvm flutter build appbundle --dart-define URL={{url}}",
		Vars: map[string]VarDef{
			"url": {
				Default:    "https://default.url",
				Positional: true,
			},
			"token": {
				Secret:  true,
				Default: "SHOULD_BE_CLEARED_BECAUSE_SECRET",
			},
		},
	}
	err := st.SaveCommand("buildApp", cmd)
	if err != nil {
		t.Fatalf("unexpected SaveCommand error: %v", err)
	}

	// Verify that secret var had Default cleared in commands.toml
	loaded, err := st.GetCommand("buildApp")
	if err != nil {
		t.Fatalf("unexpected GetCommand error: %v", err)
	}
	if loaded.Vars["token"].Default != "" {
		t.Errorf("expected secret token Default to be cleared, got %q", loaded.Vars["token"].Default)
	}

	// Save secret for buildApp
	err = st.SetSecret("buildApp", "token", "my-secret-token", false)
	if err != nil {
		t.Fatalf("unexpected SetSecret error: %v", err)
	}

	// 2. Update and rename command to buildAppNew
	cmd.Desc = "Updated description"
	err = st.UpdateCommand("buildApp", "buildAppNew", cmd)
	if err != nil {
		t.Fatalf("unexpected UpdateCommand error: %v", err)
	}

	// Old alias should not exist
	_, err = st.GetCommand("buildApp")
	if err != ErrCommandNotFound {
		t.Errorf("expected ErrCommandNotFound for old alias, got %v", err)
	}

	// New alias should exist
	newLoaded, err := st.GetCommand("buildAppNew")
	if err != nil {
		t.Fatalf("unexpected GetCommand for new alias: %v", err)
	}
	if newLoaded.Desc != "Updated description" {
		t.Errorf("Desc = %q, want 'Updated description'", newLoaded.Desc)
	}

	// Secret should have migrated to buildAppNew
	secDoc, err := st.LoadSecrets()
	if err != nil {
		t.Fatalf("unexpected LoadSecrets error: %v", err)
	}
	if _, oldExists := secDoc.Commands["buildApp"]; oldExists {
		t.Errorf("old alias still has secrets in secDoc.Commands")
	}
	if secDoc.Commands["buildAppNew"]["token"] != "my-secret-token" {
		t.Errorf("migrated secret = %q, want 'my-secret-token'", secDoc.Commands["buildAppNew"]["token"])
	}
}
