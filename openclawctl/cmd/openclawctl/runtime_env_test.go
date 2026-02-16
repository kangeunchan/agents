package main

import (
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/kangeunchan/openclawctl/internal/schema/v1"
)

func TestIsChatChannelEnabled(t *testing.T) {
	if !isChatChannelEnabled(map[string]map[string]any{
		"discord": {"enabled": true},
	}, "discord") {
		t.Fatal("expected enabled=true")
	}
	if !isChatChannelEnabled(map[string]map[string]any{
		"discord": {"enabled": "true"},
	}, "discord") {
		t.Fatal("expected enabled=\"true\"")
	}
	if isChatChannelEnabled(map[string]map[string]any{
		"discord": {"enabled": false},
	}, "discord") {
		t.Fatal("expected enabled=false")
	}
}

func TestEnsureRuntimeEnvLoadsFromDotEnv(t *testing.T) {
	t.Setenv("OPENCLAW_GATEWAY_TOKEN", "")
	t.Setenv("DISCORD_BOT_TOKEN", "")

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifast.yaml")
	if err := os.WriteFile(manifestPath, []byte("apiVersion: openclawctl/v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("OPENCLAW_GATEWAY_TOKEN=abc123\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	prev := globalOpts
	globalOpts.Manifest = manifestPath
	defer func() { globalOpts = prev }()

	manifest := &v1.Manifest{
		Gateway: v1.GatewayConfig{TokenEnv: "OPENCLAW_GATEWAY_TOKEN"},
	}
	result, err := ensureRuntimeEnv(manifest, true)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result.Values["OPENCLAW_GATEWAY_TOKEN"] != "abc123" {
		t.Fatalf("expected OPENCLAW_GATEWAY_TOKEN from .env, got %#v", result.Values["OPENCLAW_GATEWAY_TOKEN"])
	}
}

func TestEnsureRuntimeEnvRequiresDiscordTokenWhenEnabled(t *testing.T) {
	t.Setenv("OPENCLAW_GATEWAY_TOKEN", "gw-token")
	t.Setenv("DISCORD_BOT_TOKEN", "")

	prev := globalOpts
	globalOpts.Manifest = ""
	defer func() { globalOpts = prev }()

	manifest := &v1.Manifest{
		Gateway: v1.GatewayConfig{TokenEnv: "OPENCLAW_GATEWAY_TOKEN"},
		ChatChannels: map[string]map[string]any{
			"discord": {"enabled": true},
		},
	}
	_, err := ensureRuntimeEnv(manifest, true)
	if err == nil {
		t.Fatal("expected error for missing DISCORD_BOT_TOKEN")
	}
}
