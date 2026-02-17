package v1

import (
	"strings"
	"testing"
)

func validManifest() *Manifest {
	return &Manifest{
		APIVersion: "openclawctl/v1",
		Metadata:   Metadata{Name: "sample-manifest"},
		Gateway:    GatewayConfig{Mode: "local", Bind: "127.0.0.1", Port: 18789, TokenEnv: "OPENCLAW_GATEWAY_TOKEN"},
		Providers: map[string]ProviderConfig{
			"p1": {Type: "openai"},
		},
		Channels: map[string]ChannelConfig{
			"default": {Provider: "p1"},
		},
		Routing: RoutingConfig{
			DefaultChannel: "default",
		},
	}
}

func TestValidateManifestInvalidChannelProvider(t *testing.T) {
	manifest := &Manifest{
		APIVersion: "openclawctl/v1",
		Metadata:   Metadata{Name: "sample-manifest"},
		Gateway:    GatewayConfig{Mode: "local", Bind: "127.0.0.1", Port: 18789, TokenEnv: "OPENCLAW_GATEWAY_TOKEN"},
		Providers: map[string]ProviderConfig{
			"p1": {Type: "openai"},
		},
		Channels: map[string]ChannelConfig{
			"default": {Provider: "missing"},
		},
	}

	err := ValidateManifest(manifest)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "channels.default.provider") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateManifestThinkingDefaultExtraHighAlias(t *testing.T) {
	manifest := validManifest()
	manifest.Agents.Defaults.ThinkingDefault = "extra-high"

	if err := ValidateManifest(manifest); err != nil {
		t.Fatalf("expected valid manifest, got error: %v", err)
	}
}

func TestValidateManifestInvalidThinkingDefault(t *testing.T) {
	manifest := validManifest()
	manifest.Agents.Defaults.ThinkingDefault = "ultra"

	err := ValidateManifest(manifest)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "agents.defaults.thinkingDefault") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateManifestExecApprovalPreset(t *testing.T) {
	cases := []string{"full", "partial", "per-command"}
	for _, preset := range cases {
		t.Run(preset, func(t *testing.T) {
			manifest := validManifest()
			manifest.Runtime.ExecApprovalPreset = preset
			if err := ValidateManifest(manifest); err != nil {
				t.Fatalf("expected valid preset %q, got %v", preset, err)
			}
		})
	}
}

func TestValidateManifestInvalidExecApprovalPreset(t *testing.T) {
	manifest := validManifest()
	manifest.Runtime.ExecApprovalPreset = "auto"

	err := ValidateManifest(manifest)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "runtime.execApprovalPreset") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateManifestInvalidCIDR(t *testing.T) {
	manifest := &Manifest{
		APIVersion: "openclawctl/v1",
		Metadata:   Metadata{Name: "sample-manifest"},
		Gateway:    GatewayConfig{Mode: "local", Bind: "127.0.0.1", Port: 18789, TokenEnv: "OPENCLAW_GATEWAY_TOKEN"},
		Auth:       AuthConfig{AllowedCIDRs: []string{"bad-cidr"}},
	}

	err := ValidateManifest(manifest)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "auth.allowedCIDRs[0]") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateManifestPortRange(t *testing.T) {
	manifest := &Manifest{
		APIVersion: "openclawctl/v1",
		Metadata:   Metadata{Name: "sample-manifest"},
		Gateway:    GatewayConfig{Mode: "local", Bind: "127.0.0.1", Port: 70000, TokenEnv: "OPENCLAW_GATEWAY_TOKEN"},
	}

	err := ValidateManifest(manifest)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "gateway.port") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateManifestPluginsAllowDenyOverlap(t *testing.T) {
	manifest := &Manifest{
		APIVersion: "openclawctl/v1",
		Metadata:   Metadata{Name: "sample-manifest"},
		Gateway:    GatewayConfig{Mode: "local", Bind: "127.0.0.1", Port: 18789, TokenEnv: "OPENCLAW_GATEWAY_TOKEN"},
		Plugins: PluginsConfig{
			Allow: []string{"voice-call"},
			Deny:  []string{"voice-call"},
		},
	}

	err := ValidateManifest(manifest)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "allow and deny must not overlap") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateManifestSkillEnvKey(t *testing.T) {
	manifest := &Manifest{
		APIVersion: "openclawctl/v1",
		Metadata:   Metadata{Name: "sample-manifest"},
		Gateway:    GatewayConfig{Mode: "local", Bind: "127.0.0.1", Port: 18789, TokenEnv: "OPENCLAW_GATEWAY_TOKEN"},
		Skills: SkillsConfig{
			Entries: map[string]SkillConfig{
				"peekaboo": {
					Enabled: true,
					Command: "run",
					Env: map[string]string{
						"bad-key": "x",
					},
				},
			},
		},
	}

	err := ValidateManifest(manifest)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "env var key matching") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateManifestInvalidChatChannelKey(t *testing.T) {
	manifest := &Manifest{
		APIVersion: "openclawctl/v1",
		Metadata:   Metadata{Name: "sample-manifest"},
		Gateway:    GatewayConfig{Mode: "local", Bind: "127.0.0.1", Port: 18789, TokenEnv: "OPENCLAW_GATEWAY_TOKEN"},
		ChatChannels: map[string]map[string]any{
			"discord.main": {
				"enabled": true,
			},
		},
	}

	err := ValidateManifest(manifest)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "chatChannels.discord.main") {
		t.Fatalf("unexpected error: %v", err)
	}
}
