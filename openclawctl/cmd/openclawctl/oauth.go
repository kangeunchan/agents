package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type modelsListResult struct {
	Models []modelEntry `json:"models"`
}

type modelEntry struct {
	Key       string   `json:"key"`
	Available bool     `json:"available"`
	Tags      []string `json:"tags"`
}

type pluginsListResult struct {
	Plugins []pluginEntry `json:"plugins"`
}

type pluginEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Enabled     bool     `json:"enabled"`
	ProviderIDs []string `json:"providerIds"`
	Error       string   `json:"error,omitempty"`
}

type providerPreflight struct {
	Provider  string        `json:"provider"`
	Ready     bool          `json:"ready"`
	Reason    string        `json:"reason,omitempty"`
	Matching  []pluginEntry `json:"matching,omitempty"`
	Loaded    []pluginEntry `json:"loaded,omitempty"`
	Advertise bool          `json:"advertise"`
}

func runOAuth(args []string) error {
	if len(args) == 0 {
		return runOAuthLogin(nil)
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "login":
		return runOAuthLogin(args[1:])
	case "status":
		return runOAuthStatus(args[1:])
	case "providers":
		return runOAuthProviders(args[1:])
	default:
		return fmt.Errorf("unknown oauth subcommand %q (expected login|status|providers)", args[0])
	}
}

func runOAuthLogin(args []string) error {
	fs := flag.NewFlagSet("oauth login", flag.ContinueOnError)
	provider := "openai-codex"
	method := ""
	setDefault := true
	skipPreflight := false
	plain := false
	fs.StringVar(&provider, "provider", provider, "provider id for models auth login")
	fs.StringVar(&method, "method", "", "optional provider auth method id")
	fs.BoolVar(&setDefault, "set-default", true, "set provider-recommended default model on login")
	fs.BoolVar(&skipPreflight, "skip-preflight", false, "skip provider plugin preflight checks")
	fs.BoolVar(&plain, "plain", false, "disable colorized OAuth wizard output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: openclawctl oauth login [--provider <id>] [--method <id>] [--set-default] [--skip-preflight] [--plain]")
	}
	if err := expandFlagEnv("--provider", &provider); err != nil {
		return err
	}
	if err := expandFlagEnv("--method", &method); err != nil {
		return err
	}
	provider = canonicalOAuthProvider(provider)

	if !isTTY(os.Stdin) || !isTTY(os.Stdout) {
		return fmt.Errorf("oauth login requires an interactive terminal (TTY)")
	}

	manifest, err := loadManifest()
	if err != nil {
		return err
	}
	containerName := currentContainerName(manifest)

	if isBuiltInOAuthProvider(provider) {
		if !skipPreflight {
			outInfof("OAuth preflight passed: %s uses built-in OpenClaw OAuth flow", accent(provider))
		}

		cmdArgs := dockerExecInteractiveArgs(plain)
		cmdArgs = append(cmdArgs, containerName, "node", "openclaw.mjs")
		if plain {
			cmdArgs = append(cmdArgs, "--no-color")
		}
		cmdArgs = append(cmdArgs, "configure", "--section", "model")

		outInfof("Starting OAuth login in container %s", accent(containerName))
		outInfof(
			"In the wizard, choose %s",
			accent("OpenAI -> OpenAI Codex (ChatGPT OAuth)"),
		)
		if err := runDockerInteractive(context.Background(), cmdArgs); err != nil {
			return fmt.Errorf("oauth login failed: %w", err)
		}

		outInfof("OAuth login flow exited; checking authorization status")
		return runOAuthStatus([]string{"--provider", provider})
	}

	loginProvider := provider

	if !skipPreflight {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		preflight, preflightErr := evaluateProviderPreflight(ctx, containerName, provider)
		cancel()
		if preflightErr != nil {
			outWarnf("OAuth preflight skipped: %v", preflightErr)
		} else if !preflight.Ready {
			outWarnf(
				"OAuth preflight not-ready for provider %s: %s; continuing login attempt",
				accent(provider),
				preflight.Reason,
			)
		} else {
			outInfof("OAuth preflight passed: %s provider plugin is loaded", accent(provider))
		}
	}

	resolveCtx, resolveCancel := context.WithTimeout(context.Background(), 10*time.Second)
	resolvedProvider, resolveReason, resolveErr := resolveOAuthLoginProvider(resolveCtx, containerName, provider)
	resolveCancel()
	if resolveErr != nil {
		outWarnf("OAuth provider resolution skipped: %v", resolveErr)
	} else if resolvedProvider != "" {
		loginProvider = resolvedProvider
		if strings.TrimSpace(resolveReason) != "" {
			outInfof("OAuth login provider mapped: %s -> %s (%s)", accent(provider), accent(loginProvider), resolveReason)
		}
	}

	cmdArgs := dockerExecInteractiveArgs(plain)
	cmdArgs = append(cmdArgs, containerName, "node", "openclaw.mjs")
	if plain {
		cmdArgs = append(cmdArgs, "--no-color")
	}
	cmdArgs = append(cmdArgs, "models", "auth", "login")
	if strings.TrimSpace(loginProvider) != "" {
		cmdArgs = append(cmdArgs, "--provider", strings.TrimSpace(loginProvider))
	}
	if strings.TrimSpace(method) != "" {
		cmdArgs = append(cmdArgs, "--method", strings.TrimSpace(method))
	}
	if setDefault {
		cmdArgs = append(cmdArgs, "--set-default")
	}

	outInfof("Starting OAuth login in container %s", accent(containerName))
	if err := runDockerInteractive(context.Background(), cmdArgs); err != nil {
		return fmt.Errorf("oauth login failed: %w", err)
	}

	outInfof("OAuth login flow exited; checking authorization status")
	return runOAuthStatus([]string{"--provider", provider})
}

func runOAuthStatus(args []string) error {
	fs := flag.NewFlagSet("oauth status", flag.ContinueOnError)
	provider := "openai-codex"
	jsonOut := false
	fs.StringVar(&provider, "provider", provider, "provider id to inspect (prefix match)")
	fs.BoolVar(&jsonOut, "json", false, "print machine-readable JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: openclawctl oauth status [--provider <id>] [--json]")
	}
	if err := expandFlagEnv("--provider", &provider); err != nil {
		return err
	}
	provider = canonicalOAuthProvider(provider)

	manifest, err := loadManifest()
	if err != nil {
		return err
	}
	containerName := currentContainerName(manifest)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	models, err := fetchModelsList(ctx, containerName)
	if err != nil {
		return err
	}

	preflight, preflightErr := evaluateProviderPreflight(ctx, containerName, provider)
	filtered := filterModelsByProvider(models.Models, provider)
	available := anyAvailable(filtered)

	if jsonOut {
		payload := map[string]any{
			"provider": provider,
			"models":   filtered,
			"ok":       available,
		}
		if preflightErr == nil {
			payload["pluginPreflight"] = preflight
		} else {
			payload["pluginPreflightError"] = preflightErr.Error()
		}
		encoded, marshalErr := json.MarshalIndent(payload, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("encode json output: %w", marshalErr)
		}
		fmt.Println(string(encoded))
		return nil
	}

	fmt.Printf("%s %s\n", keyLabel("provider:"), accent(provider))
	if isBuiltInOAuthProvider(provider) {
		fmt.Printf("%s %s\n", keyLabel("flow:"), "built-in (OpenAI Codex OAuth)")
	} else if preflightErr == nil {
		if preflight.Ready {
			fmt.Printf("%s %s\n", keyLabel("plugin:"), stateBadge("loaded"))
		} else {
			fmt.Printf("%s %s (%s)\n", keyLabel("plugin:"), stateBadge("not-ready"), preflight.Reason)
		}
	} else {
		outWarnf("OAuth plugin preflight unavailable: %v", preflightErr)
	}

	if len(filtered) == 0 {
		outWarnf("No models found for provider %s", accent(provider))
		return nil
	}
	for _, model := range filtered {
		state := "unauthorized"
		if model.Available {
			state = "available"
		}
		fmt.Printf("  - %s %s\n", accent(model.Key), stateBadge(state))
	}
	if available {
		outSuccessf("At least one %s model is authorized", accent(provider))
		return nil
	}
	outWarnf("No %s model is authorized yet; run %s", accent(provider), accent("openclawctl oauth login --provider "+provider))
	return nil
}

func runOAuthProviders(args []string) error {
	fs := flag.NewFlagSet("oauth providers", flag.ContinueOnError)
	jsonOut := false
	provider := ""
	fs.BoolVar(&jsonOut, "json", false, "print machine-readable JSON output")
	fs.StringVar(&provider, "provider", "", "filter by provider id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: openclawctl oauth providers [--provider <id>] [--json]")
	}
	if err := expandFlagEnv("--provider", &provider); err != nil {
		return err
	}
	provider = canonicalOAuthProvider(provider)

	manifest, err := loadManifest()
	if err != nil {
		return err
	}
	containerName := currentContainerName(manifest)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	plugins, err := fetchPluginsList(ctx, containerName)
	if err != nil {
		return err
	}

	filtered := plugins.Plugins
	var preflight providerPreflight
	if strings.TrimSpace(provider) != "" {
		if isBuiltInOAuthProvider(provider) {
			preflight = builtinProviderPreflight(provider)
			filtered = nil
		} else {
			preflight = classifyProviderPlugins(filtered, provider)
			filtered = preflight.Matching
		}
	}

	if jsonOut {
		payload := map[string]any{
			"plugins": filtered,
		}
		if strings.TrimSpace(provider) != "" {
			payload["preflight"] = preflight
		}
		encoded, marshalErr := json.MarshalIndent(payload, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("encode json output: %w", marshalErr)
		}
		fmt.Println(string(encoded))
		return nil
	}

	if strings.TrimSpace(provider) != "" {
		fmt.Printf("%s %s\n", keyLabel("provider:"), accent(provider))
		fmt.Printf("%s %s\n", keyLabel("ready:"), boolBadge(preflight.Ready))
		if preflight.Reason != "" {
			fmt.Printf("%s %s\n", keyLabel("reason:"), preflight.Reason)
		}
		if isBuiltInOAuthProvider(provider) {
			fmt.Printf("%s %s\n", keyLabel("flow:"), "built-in (OpenAI Codex OAuth)")
		}
		for _, plugin := range preflight.Matching {
			fmt.Printf("  - %s %s (providerIds=%s)\n", accent(plugin.ID), stateBadge(plugin.Status), strings.Join(plugin.ProviderIDs, ","))
		}
		return nil
	}

	for _, plugin := range filtered {
		if len(plugin.ProviderIDs) == 0 {
			continue
		}
		fmt.Printf("  - %s %s (providerIds=%s)\n", accent(plugin.ID), stateBadge(plugin.Status), strings.Join(plugin.ProviderIDs, ","))
	}
	return nil
}

func runDockerInteractive(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func fetchModelsList(ctx context.Context, containerName string) (modelsListResult, error) {
	cmd := exec.CommandContext(
		ctx,
		"docker",
		"exec",
		containerName,
		"node",
		"openclaw.mjs",
		"models",
		"list",
		"--json",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return modelsListResult{}, fmt.Errorf("read models list: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	var result modelsListResult
	if err := json.Unmarshal(out, &result); err != nil {
		return modelsListResult{}, fmt.Errorf("decode models list json: %w", err)
	}
	return result, nil
}

func fetchPluginsList(ctx context.Context, containerName string) (pluginsListResult, error) {
	cmd := exec.CommandContext(
		ctx,
		"docker",
		"exec",
		containerName,
		"node",
		"openclaw.mjs",
		"plugins",
		"list",
		"--json",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return pluginsListResult{}, fmt.Errorf("read plugins list: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	var result pluginsListResult
	if err := json.Unmarshal(out, &result); err != nil {
		return pluginsListResult{}, fmt.Errorf("decode plugins list json: %w", err)
	}
	return result, nil
}

func evaluateProviderPreflight(ctx context.Context, containerName, provider string) (providerPreflight, error) {
	if isBuiltInOAuthProvider(provider) {
		return builtinProviderPreflight(provider), nil
	}
	plugins, err := fetchPluginsList(ctx, containerName)
	if err != nil {
		return providerPreflight{}, err
	}
	return classifyProviderPlugins(plugins.Plugins, provider), nil
}

func classifyProviderPlugins(plugins []pluginEntry, provider string) providerPreflight {
	provider = strings.TrimSpace(provider)
	out := providerPreflight{Provider: provider}
	if provider == "" {
		out.Reason = "provider is empty"
		return out
	}

	for _, plugin := range plugins {
		if pluginMatchesProvider(plugin, provider) {
			out.Matching = append(out.Matching, plugin)
			if pluginAdvertisesProvider(plugin, provider) {
				out.Advertise = true
			}
			if strings.EqualFold(strings.TrimSpace(plugin.Status), "loaded") || plugin.Enabled {
				out.Loaded = append(out.Loaded, plugin)
			}
		}
	}

	switch {
	case len(out.Matching) == 0:
		out.Reason = "no plugin references this provider"
	case len(out.Loaded) == 0:
		out.Reason = "matching plugins exist but none are loaded"
	default:
		out.Ready = true
	}
	return out
}

func resolveOAuthLoginProvider(ctx context.Context, containerName, requested string) (resolved string, reason string, err error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", "", nil
	}
	if isBuiltInOAuthProvider(requested) {
		return canonicalOAuthProvider(requested), "built-in provider", nil
	}

	plugins, err := fetchPluginsList(ctx, containerName)
	if err != nil {
		return "", "", err
	}

	loadedProviderIDs := map[string]string{}
	for _, plugin := range plugins.Plugins {
		loaded := strings.EqualFold(strings.TrimSpace(plugin.Status), "loaded") || plugin.Enabled
		if !loaded {
			continue
		}
		for _, providerID := range plugin.ProviderIDs {
			trimmed := strings.TrimSpace(providerID)
			if trimmed == "" {
				continue
			}
			loadedProviderIDs[strings.ToLower(trimmed)] = trimmed
		}
	}

	if len(loadedProviderIDs) == 0 {
		return requested, "no loaded provider IDs advertised by plugins", nil
	}

	requestedLower := strings.ToLower(requested)
	if exact, ok := loadedProviderIDs[requestedLower]; ok {
		return exact, "exact provider match", nil
	}

	for _, alias := range oauthProviderAliases(requested) {
		aliasLower := strings.ToLower(alias)
		if matched, ok := loadedProviderIDs[aliasLower]; ok {
			return matched, "alias match", nil
		}
	}

	return requested, "requested provider not advertised by loaded plugins", nil
}

func pluginMatchesProvider(plugin pluginEntry, provider string) bool {
	if pluginAdvertisesProvider(plugin, provider) {
		return true
	}
	normalizedProvider := normalizeToken(provider)
	if normalizedProvider == "" {
		return false
	}
	id := normalizeToken(plugin.ID)
	name := normalizeToken(plugin.Name)
	return strings.Contains(id, normalizedProvider) ||
		strings.Contains(name, normalizedProvider) ||
		strings.Contains(normalizedProvider, id)
}

func pluginAdvertisesProvider(plugin pluginEntry, provider string) bool {
	targets := oauthProviderAliases(provider)
	if len(targets) == 0 {
		return false
	}
	targetSet := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		target = strings.ToLower(strings.TrimSpace(target))
		if target == "" {
			continue
		}
		targetSet[target] = struct{}{}
	}

	for _, providerID := range plugin.ProviderIDs {
		normalized := strings.ToLower(strings.TrimSpace(providerID))
		if _, ok := targetSet[normalized]; ok {
			return true
		}
	}
	return false
}

func normalizeToken(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, "-", "")
	v = strings.ReplaceAll(v, "_", "")
	v = strings.ReplaceAll(v, " ", "")
	return v
}

func oauthProviderAliases(provider string) []string {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" {
		return nil
	}
	normalized := normalizeToken(trimmed)
	switch normalized {
	case "openaicodex":
		return []string{"openai-codex"}
	case "copilotproxy":
		return []string{"copilot-proxy"}
	default:
		return []string{trimmed}
	}
}

func canonicalOAuthProvider(provider string) string {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" {
		return ""
	}
	switch normalizeToken(trimmed) {
	case "openaicodex":
		return "openai-codex"
	case "copilotproxy":
		return "copilot-proxy"
	default:
		return trimmed
	}
}

func isBuiltInOAuthProvider(provider string) bool {
	return normalizeToken(provider) == "openaicodex"
}

func builtinProviderPreflight(provider string) providerPreflight {
	return providerPreflight{
		Provider: canonicalOAuthProvider(provider),
		Ready:    true,
		Reason:   "built-in oauth flow (openclaw configure --section model)",
	}
}

func dockerExecInteractiveArgs(plain bool) []string {
	args := []string{"exec", "-i", "-t"}
	for _, key := range []string{"TERM", "COLUMNS", "LINES", "COLORTERM", "LANG", "LC_ALL", "NO_COLOR"} {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		args = append(args, "-e", key+"="+value)
	}
	if plain {
		args = append(args, "-e", "NO_COLOR=1")
	}
	return args
}

func filterModelsByProvider(models []modelEntry, provider string) []modelEntry {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" {
		return models
	}
	prefix := trimmed + "/"
	filtered := make([]modelEntry, 0, len(models))
	for _, model := range models {
		if strings.HasPrefix(strings.ToLower(model.Key), strings.ToLower(prefix)) {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func anyAvailable(models []modelEntry) bool {
	for _, model := range models {
		if model.Available {
			return true
		}
	}
	return false
}
