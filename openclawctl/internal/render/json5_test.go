package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kangeunchan/openclawctl/internal/config"
	v1 "github.com/kangeunchan/openclawctl/internal/schema/v1"
)

func TestRenderGolden(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "testdata", "golden", "basic.yaml")
	manifest, err := config.LoadManifest(config.LoadOptions{File: manifestPath, Profile: "dev"})
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	got, err := ToJSONBytes(manifest)
	if err != nil {
		t.Fatalf("render json: %v", err)
	}

	wantPath := filepath.Join("..", "..", "testdata", "golden", "basic.json")
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	if string(got) != string(want) {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", string(got), string(want))
	}
}

func TestBuildOpenClawConfigAddsCodexOAuthProfile(t *testing.T) {
	manifest := &v1.Manifest{
		APIVersion: "openclawctl/v1",
		Metadata: v1.Metadata{
			Name: "codex",
		},
		Gateway: v1.GatewayConfig{
			Mode:     "local",
			Bind:     "127.0.0.1",
			Port:     18789,
			TokenEnv: "OPENCLAW_GATEWAY_TOKEN",
		},
		Providers: map[string]v1.ProviderConfig{
			"codex": {
				Type:  "openai-codex",
				Model: "gpt-5.3-codex",
				Options: map[string]any{
					"profileId": "openai-codex:work",
				},
			},
		},
		Channels: map[string]v1.ChannelConfig{
			"coding": {
				Provider: "codex",
				Model:    "gpt-5.3-codex",
			},
		},
		Routing: v1.RoutingConfig{
			DefaultChannel: "coding",
		},
		Logging: v1.LoggingConfig{
			Level: "info",
		},
	}

	cfg := BuildOpenClawConfig(manifest)
	agents, ok := cfg["agents"].(map[string]any)
	if !ok {
		t.Fatalf("expected agents map, got %#v", cfg["agents"])
	}
	defaults, ok := agents["defaults"].(map[string]any)
	if !ok {
		t.Fatalf("expected agents.defaults map, got %#v", agents["defaults"])
	}
	model, ok := defaults["model"].(map[string]any)
	if !ok {
		t.Fatalf("expected agents.defaults.model map, got %#v", defaults["model"])
	}
	if model["primary"] != "openai-codex/gpt-5.3-codex" {
		t.Fatalf("unexpected model primary: %#v", model["primary"])
	}

	auth, ok := cfg["auth"].(map[string]any)
	if !ok {
		t.Fatalf("expected auth map, got %#v", cfg["auth"])
	}
	profiles, ok := auth["profiles"].(map[string]any)
	if !ok {
		t.Fatalf("expected auth.profiles map, got %#v", auth["profiles"])
	}
	profile, ok := profiles["openai-codex:work"].(map[string]any)
	if !ok {
		t.Fatalf("expected oauth profile key openai-codex:work, got %#v", profiles)
	}
	if profile["provider"] != "openai-codex" || profile["mode"] != "oauth" {
		t.Fatalf("unexpected auth profile: %#v", profile)
	}
}

func TestBuildOpenClawConfigDoesNotEmbedTokenValue(t *testing.T) {
	t.Setenv("OPENCLAW_GATEWAY_TOKEN", "super-secret-token")
	manifest := &v1.Manifest{
		APIVersion: "openclawctl/v1",
		Metadata: v1.Metadata{
			Name: "no-token-leak",
		},
		Gateway: v1.GatewayConfig{
			Mode:     "local",
			Bind:     "127.0.0.1",
			Port:     18789,
			TokenEnv: "OPENCLAW_GATEWAY_TOKEN",
		},
		Logging: v1.LoggingConfig{
			Level: "info",
		},
	}
	manifest.ApplyDefaults()

	rendered, err := ToJSONBytes(manifest)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(rendered), "super-secret-token") {
		t.Fatalf("render leaked secret token: %s", string(rendered))
	}
}

func TestBuildOpenClawConfigIncludesMemorySearchToggle(t *testing.T) {
	enabled := false
	manifest := &v1.Manifest{
		APIVersion: "openclawctl/v1",
		Metadata: v1.Metadata{
			Name: "memory-search-toggle",
		},
		Gateway: v1.GatewayConfig{
			Mode:     "local",
			Bind:     "127.0.0.1",
			Port:     18789,
			TokenEnv: "OPENCLAW_GATEWAY_TOKEN",
		},
		Agents: v1.AgentsConfig{
			Defaults: v1.AgentDefaultsConfig{
				MemorySearch: v1.MemorySearchConfig{
					Enabled: &enabled,
				},
			},
		},
	}

	cfg := BuildOpenClawConfig(manifest)
	agents, ok := cfg["agents"].(map[string]any)
	if !ok {
		t.Fatalf("expected agents map, got %#v", cfg["agents"])
	}
	defaults, ok := agents["defaults"].(map[string]any)
	if !ok {
		t.Fatalf("expected agents.defaults map, got %#v", agents["defaults"])
	}
	memorySearch, ok := defaults["memorySearch"].(map[string]any)
	if !ok {
		t.Fatalf("expected agents.defaults.memorySearch map, got %#v", defaults["memorySearch"])
	}
	if memorySearch["enabled"] != false {
		t.Fatalf("expected memorySearch.enabled=false, got %#v", memorySearch["enabled"])
	}
}

func TestBuildOpenClawConfigIncludesChatChannels(t *testing.T) {
	manifest := &v1.Manifest{
		APIVersion: "openclawctl/v1",
		Metadata: v1.Metadata{
			Name: "chat-channels",
		},
		Gateway: v1.GatewayConfig{
			Mode:     "local",
			Bind:     "127.0.0.1",
			Port:     18789,
			TokenEnv: "OPENCLAW_GATEWAY_TOKEN",
		},
		ChatChannels: map[string]map[string]any{
			"discord": {
				"enabled": true,
				"token":   "${DISCORD_BOT_TOKEN}",
				"dm": map[string]any{
					"enabled": true,
					"policy":  "pairing",
				},
			},
		},
	}

	cfg := BuildOpenClawConfig(manifest)
	channels, ok := cfg["channels"].(map[string]any)
	if !ok {
		t.Fatalf("expected channels map, got %#v", cfg["channels"])
	}
	discord, ok := channels["discord"].(map[string]any)
	if !ok {
		t.Fatalf("expected channels.discord map, got %#v", channels["discord"])
	}
	if discord["token"] != "${DISCORD_BOT_TOKEN}" {
		t.Fatalf("expected channels.discord.token placeholder, got %#v", discord["token"])
	}
	dm, ok := discord["dm"].(map[string]any)
	if !ok {
		t.Fatalf("expected channels.discord.dm map, got %#v", discord["dm"])
	}
	if dm["policy"] != "pairing" {
		t.Fatalf("expected channels.discord.dm.policy=pairing, got %#v", dm["policy"])
	}
}

func TestBuildOpenClawConfigIncludesCommandsConfig(t *testing.T) {
	manifest := &v1.Manifest{
		APIVersion: "openclawctl/v1",
		Metadata: v1.Metadata{
			Name: "commands-config",
		},
		Gateway: v1.GatewayConfig{
			Mode:     "local",
			Bind:     "127.0.0.1",
			Port:     18789,
			TokenEnv: "OPENCLAW_GATEWAY_TOKEN",
		},
		Commands: map[string]any{
			"native":          "auto",
			"useAccessGroups": false,
			"allowFrom": map[string]any{
				"discord": []any{"123456789012345678"},
			},
		},
	}

	cfg := BuildOpenClawConfig(manifest)
	commands, ok := cfg["commands"].(map[string]any)
	if !ok {
		t.Fatalf("expected commands map, got %#v", cfg["commands"])
	}
	if commands["useAccessGroups"] != false {
		t.Fatalf("expected commands.useAccessGroups=false, got %#v", commands["useAccessGroups"])
	}
	allowFrom, ok := commands["allowFrom"].(map[string]any)
	if !ok {
		t.Fatalf("expected commands.allowFrom map, got %#v", commands["allowFrom"])
	}
	discordAllow, ok := allowFrom["discord"].([]any)
	if !ok || len(discordAllow) != 1 || discordAllow[0] != "123456789012345678" {
		t.Fatalf("unexpected commands.allowFrom.discord: %#v", allowFrom["discord"])
	}
}
