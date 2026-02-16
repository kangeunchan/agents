package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	v1 "github.com/kangeunchan/openclawctl/internal/schema/v1"
)

type LoadOptions struct {
	File     string
	Overlays []string
	Profile  string
}

var (
	singleEnvPlaceholderRe = regexp.MustCompile(`^\$\{([^}]+)\}$`)
	envNameTokenRe         = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func LoadManifest(opts LoadOptions) (*v1.Manifest, error) {
	if opts.File == "" {
		return nil, fmt.Errorf("manifest file is required")
	}

	files := append([]string{opts.File}, opts.Overlays...)
	merged := map[string]any{}

	for _, file := range files {
		raw, err := loadYAMLFile(file)
		if err != nil {
			return nil, err
		}
		merged = MergeMaps(merged, raw)
	}

	if err := applyEnvOverrides(merged); err != nil {
		return nil, err
	}

	resolved, err := resolveEnvPlaceholders(merged, "")
	if err != nil {
		return nil, err
	}
	resolvedMap, ok := resolved.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("resolved manifest is not an object")
	}

	manifest, err := decodeStrict(resolvedMap)
	if err != nil {
		return nil, err
	}
	manifest.ApplyDefaults()

	if err := v1.ValidateManifest(manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func loadYAMLFile(path string) (map[string]any, error) {
	cleaned := filepath.Clean(path)
	bytesData, err := os.ReadFile(cleaned)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", cleaned, err)
	}

	data, err := ParseYAML(bytesData)
	if err != nil {
		return nil, fmt.Errorf("parse yaml %s: %w", cleaned, err)
	}
	if data == nil {
		data = map[string]any{}
	}
	return data, nil
}

func decodeStrict(raw map[string]any) (*v1.Manifest, error) {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal merged manifest: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.DisallowUnknownFields()

	var manifest v1.Manifest
	if err := dec.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("strict decode: %w", err)
	}
	return &manifest, nil
}

func applyEnvOverrides(raw map[string]any) error {
	overrides := map[string]struct {
		Path  []string
		Parse func(string) (any, error)
	}{
		"OPENCLAWCTL_GATEWAY_MODE":      {Path: []string{"gateway", "mode"}, Parse: func(s string) (any, error) { return s, nil }},
		"OPENCLAWCTL_GATEWAY_BIND":      {Path: []string{"gateway", "bind"}, Parse: func(s string) (any, error) { return s, nil }},
		"OPENCLAWCTL_GATEWAY_TOKEN_ENV": {Path: []string{"gateway", "tokenEnv"}, Parse: func(s string) (any, error) { return s, nil }},
		"OPENCLAWCTL_RUNTIME_CONFIG_PATH": {
			Path:  []string{"runtime", "configPath"},
			Parse: func(s string) (any, error) { return s, nil },
		},
		"OPENCLAWCTL_GATEWAY_PORT": {
			Path: []string{"gateway", "port"},
			Parse: func(s string) (any, error) {
				v, err := strconv.Atoi(s)
				if err != nil {
					return nil, fmt.Errorf("invalid int value")
				}
				return v, nil
			},
		},
	}

	for envKey, ov := range overrides {
		value, ok := os.LookupEnv(envKey)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		parsed, err := ov.Parse(value)
		if err != nil {
			return fmt.Errorf("%s: %w", envKey, err)
		}
		SetPath(raw, ov.Path, parsed)
	}
	return nil
}

func resolveEnvPlaceholders(v any, path string) (any, error) {
	switch typed := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, value := range typed {
			resolvedKey, err := ExpandEnvString(key)
			if err != nil {
				keyPath := key
				if path != "" {
					keyPath = path + ".<key:" + key + ">"
				}
				return nil, fmt.Errorf("%w at %s", err, keyPath)
			}
			if strings.TrimSpace(resolvedKey) == "" {
				keyPath := key
				if path != "" {
					keyPath = path + ".<key:" + key + ">"
				}
				return nil, fmt.Errorf("resolved map key is empty at %s", keyPath)
			}

			nextPath := resolvedKey
			if path != "" {
				nextPath = path + "." + resolvedKey
			}
			resolved, err := resolveEnvPlaceholders(value, nextPath)
			if err != nil {
				return nil, err
			}
			if _, exists := out[resolvedKey]; exists {
				return nil, fmt.Errorf("duplicate map key %q after env expansion at %s", resolvedKey, path)
			}
			out[resolvedKey] = resolved
		}
		return out, nil
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			nextPath := fmt.Sprintf("%s[%d]", path, i)
			resolved, err := resolveEnvPlaceholders(item, nextPath)
			if err != nil {
				return nil, err
			}
			out[i] = resolved
		}
		return out, nil
	case string:
		return resolveStringPlaceholder(typed, path)
	default:
		return v, nil
	}
}

func resolveStringPlaceholder(value, path string) (string, error) {
	if isEnvNameFieldPath(path) {
		name, matched, err := parseEnvNameReference(value)
		if err != nil {
			if path == "" {
				path = "<root>"
			}
			return "", fmt.Errorf("%w at %s", err, path)
		}
		if !matched {
			if path == "" {
				path = "<root>"
			}
			return "", fmt.Errorf("env-name fields must use ${VAR} format, got %q at %s", value, path)
		}
		return name, nil
	}

	resolved, err := ExpandEnvString(value)
	if err != nil {
		if path == "" {
			path = "<root>"
		}
		return "", fmt.Errorf("%w at %s", err, path)
	}
	return resolved, nil
}

func isEnvNameFieldPath(path string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return false
	}
	last := trimmed
	if i := strings.LastIndex(last, "."); i >= 0 {
		last = last[i+1:]
	}
	return strings.HasSuffix(last, "Env")
}

func parseEnvNameReference(value string) (name string, matched bool, err error) {
	trimmed := strings.TrimSpace(value)
	parts := singleEnvPlaceholderRe.FindStringSubmatch(trimmed)
	if len(parts) != 2 {
		return "", false, nil
	}

	expr := strings.TrimSpace(parts[1])
	if strings.Contains(expr, ":-") {
		return "", true, fmt.Errorf("default syntax is not supported in env-name field %q", trimmed)
	}
	if !envNameTokenRe.MatchString(expr) {
		return "", true, fmt.Errorf("invalid env variable reference %q", trimmed)
	}
	return expr, true, nil
}
