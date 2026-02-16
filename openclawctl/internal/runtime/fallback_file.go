package runtime

import (
	"fmt"
	"os"
	"path/filepath"
)

func ReadConfigFile(path string) ([]byte, error) {
	bytesData, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}
	return bytesData, nil
}

func WriteConfigAtomically(path string, content []byte) (string, error) {
	cleaned := filepath.Clean(path)
	dir := filepath.Dir(cleaned)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}

	backupPath := ""
	if _, err := os.Stat(cleaned); err == nil {
		existing, readErr := os.ReadFile(cleaned)
		if readErr != nil {
			return "", fmt.Errorf("read existing config for backup: %w", readErr)
		}
		backupPath = fmt.Sprintf("%s.rollback.bak", cleaned)
		if writeErr := os.WriteFile(backupPath, existing, 0o600); writeErr != nil {
			return "", fmt.Errorf("write backup %s: %w", backupPath, writeErr)
		}
	}

	tmpFile, err := os.CreateTemp(dir, ".openclaw-config-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	if _, err := tmpFile.Write(content); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("chmod temp file: %w", err)
	}

	if err := os.Rename(tmpName, cleaned); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("atomic replace %s: %w", cleaned, err)
	}
	return backupPath, nil
}

func RestoreBackup(path, backupPath string) error {
	if backupPath == "" {
		return nil
	}
	backupData, err := os.ReadFile(filepath.Clean(backupPath))
	if err != nil {
		return fmt.Errorf("read backup %s: %w", backupPath, err)
	}
	if _, err := WriteConfigAtomically(path, backupData); err != nil {
		return fmt.Errorf("restore backup to %s: %w", path, err)
	}
	return nil
}

func RemoveBackup(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(filepath.Clean(path)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove backup %s: %w", path, err)
	}
	return nil
}
