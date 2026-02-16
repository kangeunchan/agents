package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/kangeunchan/openclawctl/internal/schema/v1"
)

type runtimeEnvResult struct {
	Values map[string]string
}

func ensureRuntimeEnv(manifest *v1.Manifest, requireGatewayToken bool) (runtimeEnvResult, error) {
	if manifest == nil {
		return runtimeEnvResult{}, fmt.Errorf("manifest is required")
	}

	required := []string{strings.TrimSpace(manifest.Gateway.TokenEnv)}
	optional := []string{
		"DISCORD_BOT_TOKEN",
		"OPENCLAW_GIT_USER_NAME",
		"OPENCLAW_GIT_USER_EMAIL",
		"OPENCLAW_AI_WORKTREE",
		"OPENCLAW_MAIN_REPO_PATH",
	}
	if isChatChannelEnabled(manifest.ChatChannels, "discord") {
		required = append(required, "DISCORD_BOT_TOKEN")
	}

	candidates := runtimeEnvFileCandidates()
	for _, path := range candidates {
		if err := hydrateEnvFromFile(path, append(required, optional...)); err != nil {
			return runtimeEnvResult{}, err
		}
	}

	values := map[string]string{}
	for _, key := range append(required, optional...) {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
			values[key] = value
		}
	}

	if requireGatewayToken {
		for _, key := range required {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			if strings.TrimSpace(values[key]) == "" {
				return runtimeEnvResult{}, fmt.Errorf("%s is required; set it in environment or .env", key)
			}
		}
	}

	return runtimeEnvResult{Values: values}, nil
}

func isChatChannelEnabled(channels map[string]map[string]any, name string) bool {
	cfg, ok := channels[name]
	if !ok || len(cfg) == 0 {
		return false
	}
	raw, ok := cfg["enabled"]
	if !ok {
		return false
	}
	switch typed := raw.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func runtimeEnvFileCandidates() []string {
	out := []string{}
	seen := map[string]struct{}{}

	appendIf := func(path string) {
		cleaned := filepath.Clean(path)
		if _, ok := seen[cleaned]; ok {
			return
		}
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}

	if manifestPath := strings.TrimSpace(globalOpts.Manifest); manifestPath != "" {
		manifestDir := filepath.Dir(manifestPath)
		appendIf(filepath.Join(manifestDir, ".env"))
	}
	appendIf(".env")
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		appendIf(filepath.Join(home, ".openclawctl", ".env"))
	}

	return out
}

func hydrateEnvFromFile(path string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open env file %s: %w", path, err)
	}
	defer file.Close()

	needed := map[string]struct{}{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if current, ok := os.LookupEnv(key); ok && strings.TrimSpace(current) != "" {
			continue
		}
		needed[key] = struct{}{}
	}
	if len(needed) == 0 {
		return nil
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		index := strings.Index(line, "=")
		if index <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:index])
		raw := strings.TrimSpace(line[index+1:])
		if _, ok := needed[key]; !ok {
			continue
		}

		value := strings.Trim(raw, `"'`)
		if strings.TrimSpace(value) == "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set env %s from %s: %w", key, path, err)
		}
		delete(needed, key)
		if len(needed) == 0 {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read env file %s: %w", path, err)
	}
	return nil
}
