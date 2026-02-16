package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kangeunchan/openclawctl/internal/render"
	"github.com/kangeunchan/openclawctl/internal/runtime"
	v1 "github.com/kangeunchan/openclawctl/internal/schema/v1"
)

func renderManifest(manifest *v1.Manifest) ([]byte, map[string]any, error) {
	bytesData, err := render.ToJSONBytes(manifest)
	if err != nil {
		return nil, nil, err
	}
	cfgMap, err := render.ParseJSONConfig(bytesData)
	if err != nil {
		return nil, nil, err
	}
	return bytesData, cfgMap, nil
}

func writeFile(path string, content []byte) error {
	cleaned := filepath.Clean(path)
	dir := filepath.Dir(cleaned)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	if err := os.WriteFile(cleaned, content, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", cleaned, err)
	}
	return nil
}

func resolveRuntimeConfigPath(manifest *v1.Manifest) string {
	candidate := ""
	if env := strings.TrimSpace(os.Getenv("OPENCLAW_CONFIG_PATH")); env != "" {
		candidate = env
	} else if manifest != nil && strings.TrimSpace(manifest.Runtime.ConfigPath) != "" {
		candidate = manifest.Runtime.ConfigPath
	} else {
		candidate = defaultRuntimeConfigPath()
	}
	return resolveExistingPath(candidate, 5)
}

func gatewayClient() *runtime.GatewayClient {
	return runtime.NewGatewayClient(globalOpts.GatewayRPCURL, globalOpts.GatewayToken)
}

func healthCheck(ctx context.Context, client *runtime.GatewayClient) error {
	healthCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return client.Health(healthCtx)
}

func currentContainerName(manifest *v1.Manifest) string {
	if strings.TrimSpace(globalOpts.ContainerName) != "" {
		return globalOpts.ContainerName
	}
	if manifest != nil && strings.TrimSpace(manifest.Runtime.ContainerName) != "" {
		return manifest.Runtime.ContainerName
	}
	return "openclaw-gateway"
}

func simpleUnifiedDiff(current, desired string) string {
	if current == desired {
		return ""
	}

	currentLines := strings.Split(strings.TrimSuffix(current, "\n"), "\n")
	desiredLines := strings.Split(strings.TrimSuffix(desired, "\n"), "\n")
	maxLen := len(currentLines)
	if len(desiredLines) > maxLen {
		maxLen = len(desiredLines)
	}

	var b strings.Builder
	b.WriteString("--- current\n")
	b.WriteString("+++ desired\n")

	for i := 0; i < maxLen; i++ {
		switch {
		case i >= len(currentLines):
			b.WriteString("+")
			b.WriteString(desiredLines[i])
			b.WriteString("\n")
		case i >= len(desiredLines):
			b.WriteString("-")
			b.WriteString(currentLines[i])
			b.WriteString("\n")
		case currentLines[i] != desiredLines[i]:
			b.WriteString("-")
			b.WriteString(currentLines[i])
			b.WriteString("\n")
			b.WriteString("+")
			b.WriteString(desiredLines[i])
			b.WriteString("\n")
		}
	}

	return b.String()
}

func resolveExistingPath(raw string, maxParentHops int) string {
	cleaned := filepath.Clean(raw)
	if filepath.IsAbs(cleaned) {
		return cleaned
	}
	if _, err := os.Stat(cleaned); err == nil {
		return cleaned
	}

	candidate := cleaned
	for i := 0; i < maxParentHops; i++ {
		candidate = filepath.Join("..", candidate)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return cleaned
}

func defaultRuntimeConfigPath() string {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".openclawctl", "openclaw.json")
	}
	return ".openclawctl/openclaw.json"
}

func normalizeDiffConfig(raw []byte, desired map[string]any) ([]byte, error) {
	currentCfg, err := render.ParseJSONConfig(raw)
	if err != nil {
		return raw, nil
	}
	return render.PrettyJSONFromAny(trimToDesiredShape(currentCfg, desired))
}

func trimToDesiredShape(current, desired any) any {
	switch desiredTyped := desired.(type) {
	case map[string]any:
		currentMap, ok := current.(map[string]any)
		if !ok {
			return current
		}
		out := make(map[string]any, len(desiredTyped))
		for key, desiredValue := range desiredTyped {
			currentValue, exists := currentMap[key]
			if !exists {
				continue
			}
			out[key] = trimToDesiredShape(currentValue, desiredValue)
		}
		return out
	case []any:
		currentSlice, ok := current.([]any)
		if !ok {
			return current
		}
		n := len(desiredTyped)
		if len(currentSlice) < n {
			n = len(currentSlice)
		}
		out := make([]any, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, trimToDesiredShape(currentSlice[i], desiredTyped[i]))
		}
		return out
	default:
		if currentText, ok := current.(string); ok && isRedactedSecret(currentText) {
			if desiredText, ok := desired.(string); ok && strings.TrimSpace(desiredText) != "" {
				return desiredText
			}
		}
		return current
	}
}

func isRedactedSecret(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case normalized == "":
		return false
	case strings.Contains(normalized, "redacted"):
		return true
	case normalized == "***", normalized == "****", normalized == "********":
		return true
	default:
		return false
	}
}
