package render

import (
	"encoding/json"
	"fmt"
	"strings"

	v1 "github.com/kangeunchan/openclawctl/internal/schema/v1"
)

func BuildOpenClawConfig(manifest *v1.Manifest) map[string]any {
	cfg := map[string]any{
		"gateway": buildGatewayConfig(manifest),
	}

	if auth := buildAuthConfig(manifest); len(auth) > 0 {
		cfg["auth"] = auth
	}
	if models := buildModelsConfig(manifest); len(models) > 0 {
		cfg["models"] = models
	}
	if commands := buildCommandsConfig(manifest); len(commands) > 0 {
		cfg["commands"] = commands
	}
	if channels := buildChatChannelsConfig(manifest); len(channels) > 0 {
		cfg["channels"] = channels
	}
	if agents := buildAgentsConfig(manifest); len(agents) > 0 {
		cfg["agents"] = agents
	}
	if logging := buildLoggingConfig(manifest); len(logging) > 0 {
		cfg["logging"] = logging
	}
	if plugins := buildPluginsConfig(manifest); len(plugins) > 0 {
		cfg["plugins"] = plugins
	}
	if skills := buildSkillsConfig(manifest); len(skills) > 0 {
		cfg["skills"] = skills
	}

	return cfg
}

func buildCommandsConfig(manifest *v1.Manifest) map[string]any {
	if manifest == nil || len(manifest.Commands) == 0 {
		return nil
	}
	return cloneAnyMap(manifest.Commands)
}

func buildChatChannelsConfig(manifest *v1.Manifest) map[string]any {
	if manifest == nil || len(manifest.ChatChannels) == 0 {
		return nil
	}
	out := map[string]any{}
	for channelName, channelCfg := range manifest.ChatChannels {
		if len(channelCfg) == 0 {
			continue
		}
		out[channelName] = cloneAnyValue(channelCfg)
	}
	return out
}

func buildGatewayConfig(manifest *v1.Manifest) map[string]any {
	gateway := map[string]any{
		"mode": mapGatewayMode(manifest.Gateway.Mode),
		"bind": mapGatewayBind(manifest.Gateway.Bind),
		"port": manifest.Gateway.Port,
	}

	tokenEnv := strings.TrimSpace(manifest.Auth.TokenEnv)
	if tokenEnv == "" {
		tokenEnv = strings.TrimSpace(manifest.Gateway.TokenEnv)
	}
	gatewayAuth := map[string]any{}
	if tokenEnv != "" {
		gatewayAuth["mode"] = "token"
		gatewayAuth["token"] = envExprFromName(tokenEnv)
	}
	if len(gatewayAuth) > 0 {
		gateway["auth"] = gatewayAuth
	}

	gateway["controlUi"] = map[string]any{
		"enabled":  true,
		"basePath": "/",
	}

	return gateway
}

func buildModelsConfig(manifest *v1.Manifest) map[string]any {
	if len(manifest.Providers) == 0 {
		return nil
	}

	providers := map[string]any{}
	for name, provider := range manifest.Providers {
		providerType := strings.ToLower(strings.TrimSpace(provider.Type))
		if providerType == "" || providerType == "openai-codex" {
			continue
		}

		entry := map[string]any{}
		if api := mapProviderAPI(providerType); api != "" {
			entry["api"] = api
		}

		baseURL := strings.TrimSpace(provider.Endpoint)
		if baseURL == "" {
			baseURL = getStringOption(provider.Options, "baseUrl")
		}
		if baseURL != "" {
			entry["baseUrl"] = baseURL
		}

		if apiKeyEnv := strings.TrimSpace(provider.APIKeyEnv); apiKeyEnv != "" {
			entry["apiKey"] = envExprFromName(apiKeyEnv)
		}

		models := collectProviderModels(manifest, name)
		if len(models) > 0 {
			entries := make([]map[string]any, 0, len(models))
			for _, modelID := range models {
				entries = append(entries, map[string]any{
					"id":   modelID,
					"name": modelID,
				})
			}
			entry["models"] = entries
		}

		if len(entry) > 0 {
			providers[name] = entry
		}
	}

	if len(providers) == 0 {
		return nil
	}
	return map[string]any{
		"mode":      "merge",
		"providers": providers,
	}
}

func collectProviderModels(manifest *v1.Manifest, providerName string) []string {
	models := make([]string, 0, len(manifest.Channels)+1)
	if provider, ok := manifest.Providers[providerName]; ok {
		models = appendUniqueModel(models, provider.Model)
	}
	for _, channel := range manifest.Channels {
		if strings.TrimSpace(channel.Provider) != providerName {
			continue
		}
		models = appendUniqueModel(models, channel.Model)
	}
	return models
}

func appendUniqueModel(models []string, model string) []string {
	model = strings.TrimSpace(model)
	if model == "" {
		return models
	}
	for _, item := range models {
		if item == model {
			return models
		}
	}
	return append(models, model)
}

func buildPluginsConfig(manifest *v1.Manifest) map[string]any {
	plugins := map[string]any{}
	if manifest.Plugins.Enabled {
		plugins["enabled"] = true
	}
	if len(manifest.Plugins.Allow) > 0 {
		plugins["allow"] = append([]string{}, manifest.Plugins.Allow...)
	}
	if len(manifest.Plugins.Deny) > 0 {
		plugins["deny"] = append([]string{}, manifest.Plugins.Deny...)
	}
	if len(manifest.Plugins.Entries) > 0 {
		entries := map[string]any{}
		for name, entry := range manifest.Plugins.Entries {
			item := map[string]any{}
			if entry.Enabled {
				item["enabled"] = true
			}
			if strings.TrimSpace(entry.Source) != "" {
				item["source"] = entry.Source
			}
			if strings.TrimSpace(entry.Version) != "" {
				item["version"] = entry.Version
			}
			if len(entry.Config) > 0 {
				item["config"] = cloneAnyMap(entry.Config)
			}
			entries[name] = item
		}
		plugins["entries"] = entries
	}
	return plugins
}

func buildSkillsConfig(manifest *v1.Manifest) map[string]any {
	skills := map[string]any{}
	if len(manifest.Skills.AllowBundled) > 0 {
		skills["allowBundled"] = append([]string{}, manifest.Skills.AllowBundled...)
	}
	if len(manifest.Skills.DenyBundled) > 0 {
		skills["denyBundled"] = append([]string{}, manifest.Skills.DenyBundled...)
	}
	if len(manifest.Skills.Load.ExtraDirs) > 0 {
		skills["load"] = map[string]any{
			"extraDirs": append([]string{}, manifest.Skills.Load.ExtraDirs...),
		}
	}
	if len(manifest.Skills.Entries) > 0 {
		entries := map[string]any{}
		for name, entry := range manifest.Skills.Entries {
			item := map[string]any{}
			if entry.Enabled {
				item["enabled"] = true
			}
			if strings.TrimSpace(entry.Command) != "" {
				item["command"] = entry.Command
			}
			if len(entry.Args) > 0 {
				item["args"] = append([]string{}, entry.Args...)
			}
			if len(entry.Env) > 0 {
				env := map[string]string{}
				for k, v := range entry.Env {
					env[k] = v
				}
				item["env"] = env
			}
			entries[name] = item
		}
		skills["entries"] = entries
	}
	return skills
}

func buildAuthConfig(manifest *v1.Manifest) map[string]any {
	auth := map[string]any{}

	if modelRef, providerType, profileID := resolvePrimaryModel(manifest); modelRef != "" && providerType == "openai-codex" {
		if profileID == "" {
			profileID = "openai-codex:default"
		}
		auth["profiles"] = map[string]any{
			profileID: map[string]any{
				"provider": "openai-codex",
				"mode":     "oauth",
			},
		}
		auth["order"] = map[string]any{
			"openai-codex": []string{profileID},
		}
	}

	return auth
}

func buildAgentsConfig(manifest *v1.Manifest) map[string]any {
	defaults := map[string]any{}

	modelRef, _, _ := resolvePrimaryModel(manifest)
	if modelRef != "" {
		alias := modelAliasFromRef(modelRef)
		entry := map[string]any{}
		if alias != "" {
			entry["alias"] = alias
		}
		defaults["model"] = map[string]any{
			"primary": modelRef,
		}
		defaults["models"] = map[string]any{
			modelRef: entry,
		}
	}

	if memorySearch := buildMemorySearchConfig(manifest); len(memorySearch) > 0 {
		defaults["memorySearch"] = memorySearch
	}

	if len(defaults) == 0 {
		return nil
	}
	return map[string]any{
		"defaults": defaults,
	}
}

func buildMemorySearchConfig(manifest *v1.Manifest) map[string]any {
	if manifest == nil {
		return nil
	}
	memorySearch := map[string]any{}
	if manifest.Agents.Defaults.MemorySearch.Enabled != nil {
		memorySearch["enabled"] = *manifest.Agents.Defaults.MemorySearch.Enabled
	}
	return memorySearch
}

func resolvePrimaryModel(manifest *v1.Manifest) (modelRef string, providerType string, profileID string) {
	channelName := strings.TrimSpace(manifest.Routing.DefaultChannel)
	if channelName == "" && len(manifest.Channels) == 1 {
		for key := range manifest.Channels {
			channelName = key
		}
	}
	channel, ok := manifest.Channels[channelName]
	if !ok {
		return "", "", ""
	}

	providerName := strings.TrimSpace(channel.Provider)
	if providerName == "" {
		return "", "", ""
	}
	provider, ok := manifest.Providers[providerName]
	if !ok {
		return "", "", ""
	}

	providerType = strings.ToLower(strings.TrimSpace(provider.Type))
	modelID := strings.TrimSpace(channel.Model)
	if modelID == "" {
		modelID = strings.TrimSpace(provider.Model)
	}
	if providerType == "" || modelID == "" {
		return "", "", ""
	}

	profileID = getStringOption(provider.Options, "profileId")
	return providerType + "/" + modelID, providerType, profileID
}

func getStringOption(options map[string]any, key string) string {
	if len(options) == 0 {
		return ""
	}
	raw, ok := options[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func buildLoggingConfig(manifest *v1.Manifest) map[string]any {
	logging := map[string]any{}

	if level := strings.TrimSpace(manifest.Logging.Level); level != "" {
		logging["level"] = level
	}

	if style := mapConsoleStyle(manifest.Logging.Format, manifest.Logging.Structured); style != "" {
		logging["consoleStyle"] = style
	}

	return logging
}

func mapConsoleStyle(format string, structured bool) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return "json"
	case "text":
		return "pretty"
	case "":
		if structured {
			return "json"
		}
		return ""
	default:
		return ""
	}
}

func mapProviderAPI(providerType string) string {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "openai", "openai-compatible":
		return "openai"
	case "anthropic":
		return "anthropic"
	default:
		return ""
	}
}

func envExprFromName(envName string) string {
	trimmed := strings.TrimSpace(envName)
	if trimmed == "" {
		return ""
	}
	return "${" + trimmed + "}"
}

func modelAliasFromRef(modelRef string) string {
	trimmed := strings.TrimSpace(modelRef)
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func mapGatewayMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "managed", "remote":
		return "remote"
	default:
		return "local"
	}
}

func mapGatewayBind(bind string) string {
	raw := strings.TrimSpace(bind)
	normalized := strings.ToLower(raw)
	switch normalized {
	case "", "localhost", "127.0.0.1", "::1", "loopback":
		return "loopback"
	case "0.0.0.0", "lan", "all":
		return "lan"
	case "tailnet":
		return "tailnet"
	default:
		return raw
	}
}

func ToJSONBytes(manifest *v1.Manifest) ([]byte, error) {
	cfg := BuildOpenClawConfig(manifest)
	bytesData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return append(bytesData, '\n'), nil
}

func ParseJSONConfig(bytesData []byte) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal(bytesData, &out); err != nil {
		return nil, fmt.Errorf("unmarshal config json: %w", err)
	}
	return out, nil
}

func PrettyJSONFromAny(v any) ([]byte, error) {
	bytesData, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return append(bytesData, '\n'), nil
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAnyValue(v)
	}
	return out
}

func cloneAnyValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case map[string]string:
		cloned := make(map[string]string, len(typed))
		for k, value := range typed {
			cloned[k] = value
		}
		return cloned
	case []any:
		cloned := make([]any, len(typed))
		for i, value := range typed {
			cloned[i] = cloneAnyValue(value)
		}
		return cloned
	default:
		return v
	}
}
