package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteConfigAtomicallyCreatesSingleRollbackBackup(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "openclaw.json")
	if err := os.WriteFile(configPath, []byte(`{"old":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	backupPath, err := WriteConfigAtomically(configPath, []byte(`{"new":true}`))
	if err != nil {
		t.Fatalf("write config: %v", err)
	}
	if backupPath == "" {
		t.Fatal("expected backup path")
	}
	if backupPath != configPath+".rollback.bak" {
		t.Fatalf("unexpected backup path: %s", backupPath)
	}

	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backupData) != `{"old":true}` {
		t.Fatalf("unexpected backup data: %s", string(backupData))
	}
}

func TestRemoveBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.rollback.bak")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := RemoveBackup(path); err != nil {
		t.Fatalf("remove backup: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected backup removed, stat err=%v", err)
	}

	if err := RemoveBackup(path); err != nil {
		t.Fatalf("remove missing backup should be no-op, got: %v", err)
	}
}
