package v1

import (
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"slices"
	"strings"
)

var (
	manifestNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{1,62}$`)
	keyNameRe      = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`)
	envVarNameRe   = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
)

type ValidationError struct {
	Path     string
	Expected string
	Actual   any
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: expected %s, got %v", e.Path, e.Expected, e.Actual)
}

type ValidationErrors struct {
	Errors []ValidationError
}

func (e *ValidationErrors) Add(path, expected string, actual any) {
	e.Errors = append(e.Errors, ValidationError{Path: path, Expected: expected, Actual: actual})
}

func (e ValidationErrors) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, item := range e.Errors {
		parts = append(parts, item.Error())
	}
	return strings.Join(parts, "; ")
}

func (e ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

func ValidateManifest(m *Manifest) error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}

	errs := &ValidationErrors{}

	if m.APIVersion != "openclawctl/v1" {
		errs.Add("apiVersion", "openclawctl/v1", m.APIVersion)
	}
	if m.Metadata.Name == "" {
		errs.Add("metadata.name", "non-empty string", m.Metadata.Name)
	} else if !manifestNameRe.MatchString(m.Metadata.Name) {
		errs.Add("metadata.name", "alphanumeric slug (2-63 chars)", m.Metadata.Name)
	}

	switch m.Gateway.Mode {
	case "local", "managed":
	default:
		errs.Add("gateway.mode", "one of [local, managed]", m.Gateway.Mode)
	}

	if m.Gateway.Port <= 0 || m.Gateway.Port > 65535 {
		errs.Add("gateway.port", "1..65535", m.Gateway.Port)
	}

	if m.Gateway.TokenEnv == "" {
		errs.Add("gateway.tokenEnv", "non-empty env var name", m.Gateway.TokenEnv)
	}

	if !isLoopbackBinding(m.Gateway.Bind) && m.Auth.TokenEnv == "" && m.Gateway.TokenEnv == "" {
		errs.Add("auth.tokenEnv", "required when gateway.bind is non-loopback", m.Auth.TokenEnv)
	}

	if m.Logging.Level != "" {
		switch m.Logging.Level {
		case "trace", "debug", "info", "warn", "error":
		default:
			errs.Add("logging.level", "one of [trace, debug, info, warn, error]", m.Logging.Level)
		}
	}

	if m.Logging.Format != "" {
		switch m.Logging.Format {
		case "json", "text":
		default:
			errs.Add("logging.format", "one of [json, text]", m.Logging.Format)
		}
	}

	for name, provider := range m.Providers {
		if !keyNameRe.MatchString(name) {
			errs.Add("providers."+name, "provider key matching ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$", name)
		}
		if provider.Type == "" {
			errs.Add("providers."+name+".type", "non-empty string", provider.Type)
		}
		if provider.Endpoint != "" && !strings.HasPrefix(provider.Endpoint, "http://") && !strings.HasPrefix(provider.Endpoint, "https://") {
			errs.Add("providers."+name+".endpoint", "URL starting with http:// or https://", provider.Endpoint)
		}
		validateRetry(errs, "providers."+name+".retry", provider.Retry)
		validateTimeout(errs, "providers."+name+".timeout", provider.Timeout)
	}

	for name, channel := range m.Channels {
		if !keyNameRe.MatchString(name) {
			errs.Add("channels."+name, "channel key matching ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$", name)
		}
		if channel.Provider == "" {
			errs.Add("channels."+name+".provider", "non-empty provider reference", channel.Provider)
		} else if _, ok := m.Providers[channel.Provider]; !ok {
			errs.Add("channels."+name+".provider", "existing provider key", channel.Provider)
		}
		if channel.MaxTokens < 0 {
			errs.Add("channels."+name+".maxTokens", "greater than or equal to 0", channel.MaxTokens)
		}
		if channel.Temperature < 0 || channel.Temperature > 2 {
			errs.Add("channels."+name+".temperature", "range 0..2", channel.Temperature)
		}
		validateRetry(errs, "channels."+name+".retry", channel.Retry)
		validateTimeout(errs, "channels."+name+".timeout", channel.Timeout)
	}

	for name := range m.ChatChannels {
		if !keyNameRe.MatchString(name) {
			errs.Add("chatChannels."+name, "channel key matching ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$", name)
		}
	}

	if m.Routing.DefaultChannel != "" {
		if _, ok := m.Channels[m.Routing.DefaultChannel]; !ok {
			errs.Add("routing.defaultChannel", "existing channel key", m.Routing.DefaultChannel)
		}
	}

	for i, rule := range m.Routing.Rules {
		path := fmt.Sprintf("routing.rules[%d]", i)
		if rule.UseChannel == "" {
			errs.Add(path+".useChannel", "non-empty channel key", rule.UseChannel)
		} else if _, ok := m.Channels[rule.UseChannel]; !ok {
			errs.Add(path+".useChannel", "existing channel key", rule.UseChannel)
		}
		for j, fallback := range rule.Fallbacks {
			if _, ok := m.Channels[fallback]; !ok {
				errs.Add(fmt.Sprintf("%s.fallbacks[%d]", path, j), "existing channel key", fallback)
			}
		}
	}

	for i, cidr := range m.Auth.AllowedCIDRs {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			errs.Add(fmt.Sprintf("auth.allowedCIDRs[%d]", i), "valid CIDR prefix", cidr)
		}
	}

	if thinking := strings.TrimSpace(m.Agents.Defaults.ThinkingDefault); thinking != "" {
		if normalizeThinkingLevel(thinking) == "" {
			errs.Add("agents.defaults.thinkingDefault", "one of [off, minimal, low, medium, high, xhigh, extra-high]", thinking)
		}
	}

	if preset := strings.TrimSpace(m.Runtime.ExecApprovalPreset); preset != "" {
		switch strings.ToLower(preset) {
		case "full", "partial", "per-command":
		default:
			errs.Add("runtime.execApprovalPreset", "one of [full, partial, per-command]", preset)
		}
	}

	validatePlugins(errs, &m.Plugins)
	validateSkills(errs, &m.Skills)

	if m.Gateway.Health.IntervalSeconds < 0 {
		errs.Add("gateway.health.intervalSeconds", "greater than or equal to 0", m.Gateway.Health.IntervalSeconds)
	}
	if m.Gateway.Health.TimeoutSeconds < 0 {
		errs.Add("gateway.health.timeoutSeconds", "greater than or equal to 0", m.Gateway.Health.TimeoutSeconds)
	}
	if m.Gateway.Health.FailureThreshold < 0 {
		errs.Add("gateway.health.failureThreshold", "greater than or equal to 0", m.Gateway.Health.FailureThreshold)
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

func validateRetry(errs *ValidationErrors, path string, retry RetryConfig) {
	if retry.Attempts < 0 {
		errs.Add(path+".attempts", "greater than or equal to 0", retry.Attempts)
	}
	if retry.InitialMillis < 0 {
		errs.Add(path+".initialMillis", "greater than or equal to 0", retry.InitialMillis)
	}
	if retry.MaxMillis < 0 {
		errs.Add(path+".maxMillis", "greater than or equal to 0", retry.MaxMillis)
	}
	if retry.MaxMillis > 0 && retry.InitialMillis > retry.MaxMillis {
		errs.Add(path+".maxMillis", "greater than or equal to initialMillis", retry.MaxMillis)
	}
}

func validateTimeout(errs *ValidationErrors, path string, timeout TimeoutConfig) {
	if timeout.ConnectSeconds < 0 {
		errs.Add(path+".connectSeconds", "greater than or equal to 0", timeout.ConnectSeconds)
	}
	if timeout.ReadSeconds < 0 {
		errs.Add(path+".readSeconds", "greater than or equal to 0", timeout.ReadSeconds)
	}
	if timeout.TotalSeconds < 0 {
		errs.Add(path+".totalSeconds", "greater than or equal to 0", timeout.TotalSeconds)
	}
}

func isLoopbackBinding(bind string) bool {
	if bind == "" {
		return true
	}
	if bind == "localhost" {
		return true
	}
	if ip, err := netip.ParseAddr(bind); err == nil {
		return ip.IsLoopback()
	}
	host, _, err := net.SplitHostPort(bind)
	if err == nil {
		if host == "localhost" {
			return true
		}
		if ip, parseErr := netip.ParseAddr(host); parseErr == nil {
			return ip.IsLoopback()
		}
	}
	return false
}

func validatePlugins(errs *ValidationErrors, plugins *PluginsConfig) {
	if plugins == nil {
		return
	}

	allow := normalizeNameList(errs, "plugins.allow", plugins.Allow)
	deny := normalizeNameList(errs, "plugins.deny", plugins.Deny)
	for _, name := range allow {
		if slices.Contains(deny, name) {
			errs.Add("plugins", "allow and deny must not overlap", name)
		}
	}

	for name, entry := range plugins.Entries {
		if !keyNameRe.MatchString(name) {
			errs.Add("plugins.entries."+name, "plugin key matching ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$", name)
		}
		if strings.TrimSpace(entry.Source) == "" && strings.TrimSpace(entry.Version) != "" {
			errs.Add("plugins.entries."+name+".source", "required when version is set", entry.Source)
		}
	}
}

func validateSkills(errs *ValidationErrors, skills *SkillsConfig) {
	if skills == nil {
		return
	}

	allow := normalizeNameList(errs, "skills.allowBundled", skills.AllowBundled)
	deny := normalizeNameList(errs, "skills.denyBundled", skills.DenyBundled)
	for _, name := range allow {
		if slices.Contains(deny, name) {
			errs.Add("skills", "allowBundled and denyBundled must not overlap", name)
		}
	}

	for i, dir := range skills.Load.ExtraDirs {
		if strings.TrimSpace(dir) == "" {
			errs.Add(fmt.Sprintf("skills.load.extraDirs[%d]", i), "non-empty path", dir)
		}
	}

	for name, entry := range skills.Entries {
		if !keyNameRe.MatchString(name) {
			errs.Add("skills.entries."+name, "skill key matching ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$", name)
		}
		if strings.TrimSpace(entry.Command) == "" && len(entry.Args) > 0 {
			errs.Add("skills.entries."+name+".command", "required when args are set", entry.Command)
		}
		for i, arg := range entry.Args {
			if strings.TrimSpace(arg) == "" {
				errs.Add(fmt.Sprintf("skills.entries.%s.args[%d]", name, i), "non-empty string", arg)
			}
		}
		for envKey, envValue := range entry.Env {
			if !envVarNameRe.MatchString(envKey) {
				errs.Add("skills.entries."+name+".env."+envKey, "env var key matching ^[A-Z_][A-Z0-9_]*$", envKey)
			}
			if strings.TrimSpace(envValue) == "" {
				errs.Add("skills.entries."+name+".env."+envKey, "non-empty string", envValue)
			}
		}
	}
}

func normalizeNameList(errs *ValidationErrors, path string, values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for i, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			errs.Add(fmt.Sprintf("%s[%d]", path, i), "non-empty key name", value)
			continue
		}
		if !keyNameRe.MatchString(normalized) {
			errs.Add(fmt.Sprintf("%s[%d]", path, i), "key matching ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$", normalized)
			continue
		}
		if _, exists := seen[normalized]; exists {
			errs.Add(path, "no duplicate values", normalized)
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeThinkingLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "off", "minimal", "low", "medium", "high", "xhigh":
		return strings.ToLower(strings.TrimSpace(level))
	case "extra-high", "extra_high", "extrahigh":
		return "xhigh"
	default:
		return ""
	}
}
