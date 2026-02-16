package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifestStrictUnknownKey(t *testing.T) {
	dir := t.TempDir()
	manifest := `
apiVersion: openclawctl/v1
metadata:
  name: test-manifest
gateway:
  mode: local
  bind: 127.0.0.1
  port: 18789
  tokenEnv: ${OPENCLAW_GATEWAY_TOKEN}
unknownTop: true
`
	path := filepath.Join(dir, "base.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(LoadOptions{File: path, Profile: "dev"})
	if err == nil {
		t.Fatal("expected strict decode error for unknown key")
	}
	if !strings.Contains(err.Error(), "unknown field \"unknownTop\"") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadManifestEnvPlaceholderSuccess(t *testing.T) {
	t.Setenv("OPENCLAW_GATEWAY_TOKEN", "secret-value")
	dir := t.TempDir()
	manifest := `
apiVersion: openclawctl/v1
metadata:
  name: test-manifest
gateway:
  mode: local
  bind: 127.0.0.1
  port: 18789
  tokenEnv: ${OPENCLAW_GATEWAY_TOKEN}
channels:
  default:
    provider: local
    headers:
      Authorization: Bearer ${OPENCLAW_GATEWAY_TOKEN}
providers:
  local:
    type: openai
`
	path := filepath.Join(dir, "base.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadManifest(LoadOptions{File: path, Profile: "dev"})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	auth, ok := loaded.Channels["default"].Headers["Authorization"].(string)
	if !ok {
		t.Fatalf("expected authorization header as string, got %#v", loaded.Channels["default"].Headers["Authorization"])
	}
	if auth != "Bearer secret-value" {
		t.Fatalf("unexpected resolved value: %s", auth)
	}
}

func TestLoadManifestEnvPlaceholderMissing(t *testing.T) {
	dir := t.TempDir()
	manifest := `
apiVersion: openclawctl/v1
metadata:
  name: test-manifest
gateway:
  mode: local
  bind: 127.0.0.1
  port: 18789
  tokenEnv: ${OPENCLAW_GATEWAY_TOKEN}
logging:
  fields:
    token: ${UNSET_TOKEN}
`
	path := filepath.Join(dir, "base.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(LoadOptions{File: path, Profile: "dev"})
	if err == nil {
		t.Fatal("expected unresolved env error")
	}
	if !strings.Contains(err.Error(), "UNSET_TOKEN") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadManifestEnvPlaceholderDefault(t *testing.T) {
	dir := t.TempDir()
	manifest := `
apiVersion: openclawctl/v1
metadata:
  name: test-manifest
gateway:
  mode: local
  bind: 127.0.0.1
  port: 18789
  tokenEnv: ${OPENCLAW_GATEWAY_TOKEN}
logging:
  fields:
    region: ${OPENCLAW_REGION:-us-east-1}
`
	path := filepath.Join(dir, "base.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadManifest(LoadOptions{File: path, Profile: "dev"})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	got, ok := loaded.Logging.Fields["region"].(string)
	if !ok {
		t.Fatalf("expected region string, got %#v", loaded.Logging.Fields["region"])
	}
	if got != "us-east-1" {
		t.Fatalf("expected default region us-east-1, got %q", got)
	}
}

func TestLoadManifestChatChannelsWithEnvExpansion(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "discord-secret-token")
	dir := t.TempDir()
	manifest := `
apiVersion: openclawctl/v1
metadata:
  name: test-manifest
gateway:
  mode: local
  bind: 127.0.0.1
  port: 18789
  tokenEnv: ${OPENCLAW_GATEWAY_TOKEN}
chatChannels:
  discord:
    enabled: true
    token: ${DISCORD_BOT_TOKEN}
    dm:
      enabled: true
      policy: pairing
`
	path := filepath.Join(dir, "base.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadManifest(LoadOptions{File: path, Profile: "dev"})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	discord, ok := loaded.ChatChannels["discord"]
	if !ok {
		t.Fatalf("expected chatChannels.discord, got %#v", loaded.ChatChannels)
	}
	token, ok := discord["token"].(string)
	if !ok {
		t.Fatalf("expected chatChannels.discord.token as string, got %#v", discord["token"])
	}
	if token != "discord-secret-token" {
		t.Fatalf("unexpected token value: %q", token)
	}
}

func TestLoadManifestOverlayPrecedence(t *testing.T) {
	dir := t.TempDir()
	base := `
apiVersion: openclawctl/v1
metadata:
  name: test-manifest
gateway:
  mode: local
  bind: 127.0.0.1
  port: 18789
  tokenEnv: ${OPENCLAW_GATEWAY_TOKEN}
logging:
  level: info
`
	overlay := `
logging:
  level: debug
`
	basePath := filepath.Join(dir, "base.yaml")
	overlayPath := filepath.Join(dir, "dev.yaml")
	if err := os.WriteFile(basePath, []byte(base), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlayPath, []byte(overlay), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadManifest(LoadOptions{File: basePath, Overlays: []string{overlayPath}, Profile: "dev"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.Logging.Level != "debug" {
		t.Fatalf("expected overlay logging.level=debug, got %s", loaded.Logging.Level)
	}

	t.Setenv("OPENCLAWCTL_GATEWAY_PORT", "19999")
	envLoaded, err := LoadManifest(LoadOptions{File: basePath, Overlays: []string{overlayPath}, Profile: "dev"})
	if err != nil {
		t.Fatalf("unexpected error with env override: %v", err)
	}
	if envLoaded.Gateway.Port != 19999 {
		t.Fatalf("expected env override gateway.port=19999, got %d", envLoaded.Gateway.Port)
	}
}

func TestLoadManifestDuplicateKeyFails(t *testing.T) {
	dir := t.TempDir()
	manifest := `
apiVersion: openclawctl/v1
metadata:
  name: test-manifest
gateway:
  mode: local
  mode: managed
  bind: 127.0.0.1
  port: 18789
  tokenEnv: ${OPENCLAW_GATEWAY_TOKEN}
`
	path := filepath.Join(dir, "base.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(LoadOptions{File: path, Profile: "dev"})
	if err == nil {
		t.Fatal("expected duplicate key error")
	}
	if !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadManifestEnvNameFieldSupportsPlaceholderSyntax(t *testing.T) {
	dir := t.TempDir()
	manifest := `
apiVersion: openclawctl/v1
metadata:
  name: test-manifest
gateway:
  mode: local
  bind: 127.0.0.1
  port: 18789
  tokenEnv: ${OPENCLAW_GATEWAY_TOKEN}
providers:
  primary:
    type: openai
    apiKeyEnv: ${OPENAI_API_KEY}
channels:
  default:
    provider: primary
`
	path := filepath.Join(dir, "base.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadManifest(LoadOptions{File: path, Profile: "dev"})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if loaded.Gateway.TokenEnv != "OPENCLAW_GATEWAY_TOKEN" {
		t.Fatalf("unexpected gateway.tokenEnv: %q", loaded.Gateway.TokenEnv)
	}
	if loaded.Providers["primary"].APIKeyEnv != "OPENAI_API_KEY" {
		t.Fatalf("unexpected providers.primary.apiKeyEnv: %q", loaded.Providers["primary"].APIKeyEnv)
	}
}

func TestLoadManifestEnvPlaceholderInMapKey(t *testing.T) {
	t.Setenv("OPENCLAW_DISCORD_GUILD_ID", "123456789012345678")
	t.Setenv("OPENCLAW_DISCORD_CHANNEL_ID", "234567890123456789")

	dir := t.TempDir()
	manifest := `
apiVersion: openclawctl/v1
metadata:
  name: test-manifest
gateway:
  mode: local
  bind: 127.0.0.1
  port: 18789
  tokenEnv: ${OPENCLAW_GATEWAY_TOKEN}
chatChannels:
  discord:
    enabled: true
    groupPolicy: allowlist
    guilds:
      ${OPENCLAW_DISCORD_GUILD_ID}:
        channels:
          ${OPENCLAW_DISCORD_CHANNEL_ID}:
            allow: true
`
	path := filepath.Join(dir, "base.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadManifest(LoadOptions{File: path, Profile: "dev"})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	discord, ok := loaded.ChatChannels["discord"]
	if !ok {
		t.Fatalf("expected chatChannels.discord, got %#v", loaded.ChatChannels)
	}
	guildsRaw, ok := discord["guilds"].(map[string]any)
	if !ok {
		t.Fatalf("expected guilds map, got %#v", discord["guilds"])
	}
	guildRaw, ok := guildsRaw["123456789012345678"].(map[string]any)
	if !ok {
		t.Fatalf("expected expanded guild key, got %#v", guildsRaw)
	}
	channelsRaw, ok := guildRaw["channels"].(map[string]any)
	if !ok {
		t.Fatalf("expected channels map, got %#v", guildRaw["channels"])
	}
	if _, ok := channelsRaw["234567890123456789"]; !ok {
		t.Fatalf("expected expanded channel key, got %#v", channelsRaw)
	}
}

func TestLoadManifestEnvPlaceholderMissingInMapKey(t *testing.T) {
	dir := t.TempDir()
	manifest := `
apiVersion: openclawctl/v1
metadata:
  name: test-manifest
gateway:
  mode: local
  bind: 127.0.0.1
  port: 18789
  tokenEnv: ${OPENCLAW_GATEWAY_TOKEN}
chatChannels:
  discord:
    enabled: true
    groupPolicy: allowlist
    guilds:
      ${OPENCLAW_DISCORD_GUILD_ID}:
        channels:
          "111":
            allow: true
`
	path := filepath.Join(dir, "base.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(LoadOptions{File: path, Profile: "dev"})
	if err == nil {
		t.Fatal("expected unresolved env error for map key")
	}
	if !strings.Contains(err.Error(), "OPENCLAW_DISCORD_GUILD_ID") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadManifestEnvNameFieldRejectsPlainString(t *testing.T) {
	dir := t.TempDir()
	manifest := `
apiVersion: openclawctl/v1
metadata:
  name: test-manifest
gateway:
  mode: local
  bind: 127.0.0.1
  port: 18789
  tokenEnv: OPENCLAW_GATEWAY_TOKEN
`
	path := filepath.Join(dir, "base.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(LoadOptions{File: path, Profile: "dev"})
	if err == nil {
		t.Fatal("expected plain env-name format error")
	}
	if !strings.Contains(err.Error(), "must use ${VAR} format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadManifestEnvNameFieldRejectsDefaultPlaceholder(t *testing.T) {
	dir := t.TempDir()
	manifest := `
apiVersion: openclawctl/v1
metadata:
  name: test-manifest
gateway:
  mode: local
  bind: 127.0.0.1
  port: 18789
  tokenEnv: ${OPENCLAW_GATEWAY_TOKEN:-fallback}
`
	path := filepath.Join(dir, "base.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(LoadOptions{File: path, Profile: "dev"})
	if err == nil {
		t.Fatal("expected invalid env-name placeholder error")
	}
	if !strings.Contains(err.Error(), "default syntax is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}
