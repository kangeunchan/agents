package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kangeunchan/openclawctl/internal/config"
	v1 "github.com/kangeunchan/openclawctl/internal/schema/v1"
	"github.com/kangeunchan/openclawctl/internal/version"
)

type stringSliceValue []string

func (s *stringSliceValue) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceValue) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type GlobalOptions struct {
	Manifest      string
	Overlays      []string
	Profile       string
	GatewayRPCURL string
	GatewayToken  string
	ContainerName string
}

var globalOpts GlobalOptions

func parseRootArgs(args []string) (command string, commandArgs []string, showVersion bool, err error) {
	globalOpts = GlobalOptions{
		Manifest:      defaultManifestPath(),
		Profile:       envOr("OPENCLAWCTL_PROFILE", "dev"),
		GatewayRPCURL: envOr("OPENCLAW_GATEWAY_URL", envOr("OPENCLAW_GATEWAY_RPC_URL", "ws://127.0.0.1:18789")),
		GatewayToken:  os.Getenv("OPENCLAW_GATEWAY_TOKEN"),
		ContainerName: envOr("OPENCLAW_CONTAINER_NAME", "openclaw-gateway"),
	}

	var overlays stringSliceValue

	fs := flag.NewFlagSet("openclawctl", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&globalOpts.Manifest, "file", globalOpts.Manifest, "base manifest file")
	fs.StringVar(&globalOpts.Manifest, "f", globalOpts.Manifest, "base manifest file")
	fs.Var(&overlays, "overlay", "overlay manifest files (repeatable)")
	fs.StringVar(&globalOpts.Profile, "profile", globalOpts.Profile, "runtime profile (dev|prod)")
	fs.StringVar(&globalOpts.GatewayRPCURL, "gateway-rpc-url", globalOpts.GatewayRPCURL, "OpenClaw gateway URL (ws/http)")
	fs.StringVar(&globalOpts.GatewayToken, "gateway-token", globalOpts.GatewayToken, "OpenClaw gateway token")
	fs.StringVar(&globalOpts.ContainerName, "container-name", globalOpts.ContainerName, "Docker container name")
	fs.BoolVar(&showVersion, "version", false, "print version")

	if err := fs.Parse(args); err != nil {
		return "", nil, false, err
	}

	globalOpts.Overlays = append(globalOpts.Overlays, overlays...)
	if err := expandGlobalEnvInputs(); err != nil {
		return "", nil, false, err
	}

	rest := fs.Args()
	if len(rest) == 0 {
		return "", nil, showVersion, nil
	}
	return rest[0], rest[1:], showVersion, nil
}

func loadManifest() (*v1.Manifest, error) {
	if strings.TrimSpace(globalOpts.Manifest) == "" {
		return nil, fmt.Errorf("--file is required")
	}
	return config.LoadManifest(config.LoadOptions{
		File:     globalOpts.Manifest,
		Overlays: globalOpts.Overlays,
		Profile:  globalOpts.Profile,
	})
}

func usage() string {
	return `openclawctl - YAML-first OpenClaw configuration controller

Usage:
  openclawctl [global flags] <command> [command flags]

Global flags:
  --file, -f             Base manifest file
  --overlay              Overlay manifest file (repeatable)
  --profile              Runtime profile (dev|prod)
  --gateway-rpc-url      OpenClaw gateway URL (ws/http)
  --gateway-token        OpenClaw gateway token
  --container-name       Docker container name
  --version              Print version

Commands:
  validate               Validate manifest
  render                 Render OpenClaw JSON config
  diff                   Diff desired config against live/file config
  apply                  Apply config to running OpenClaw
  status                 Show health status (supports -o/--observe)
  up                     Start or replace runtime
  down                   Stop runtime
  logs                   Stream runtime logs
  devices                Device pairing requests (list|approve|reject)
  oauth                  OAuth auth helpers (login|status|providers)
  version                Print version
`
}

func defaultManifestPath() string {
	if env := os.Getenv("OPENCLAWCTL_MANIFEST"); strings.TrimSpace(env) != "" {
		return env
	}
	candidates := []string{
		"openclaw/manifast.yaml",
		"./openclaw/manifast.yaml",
		"../openclaw/manifast.yaml",
		"openclaw/manifests.yaml",
		"./openclaw/manifests.yaml",
		"../openclaw/manifests.yaml",
		"openclaw/manifests/base.yaml",
		"./openclaw/manifests/base.yaml",
		"../openclaw/manifests/base.yaml",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Clean(candidate)); err == nil {
			return candidate
		}
	}
	return "openclaw/manifast.yaml"
}

func envOr(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func buildVersion() string {
	return fmt.Sprintf("%s (%s, %s)", version.Version, version.Commit, version.Date)
}

func expandGlobalEnvInputs() error {
	expand := func(name string, value *string) error {
		resolved, err := config.ExpandEnvString(*value)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		*value = resolved
		return nil
	}

	if err := expand("--file", &globalOpts.Manifest); err != nil {
		return err
	}
	if err := expand("--profile", &globalOpts.Profile); err != nil {
		return err
	}
	if err := expand("--gateway-rpc-url", &globalOpts.GatewayRPCURL); err != nil {
		return err
	}
	if err := expand("--gateway-token", &globalOpts.GatewayToken); err != nil {
		return err
	}
	if err := expand("--container-name", &globalOpts.ContainerName); err != nil {
		return err
	}
	for i := range globalOpts.Overlays {
		resolved, err := config.ExpandEnvString(globalOpts.Overlays[i])
		if err != nil {
			return fmt.Errorf("--overlay[%d]: %w", i, err)
		}
		globalOpts.Overlays[i] = resolved
	}
	return nil
}
