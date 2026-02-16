package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	envPlaceholderExprRe = regexp.MustCompile(`\$\{([^}]+)\}`)
	envNameRe            = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// ExpandEnvString replaces ${VAR} and ${VAR:-default} placeholders
// using process environment variables.
func ExpandEnvString(input string) (string, error) {
	if input == "" {
		return input, nil
	}

	matches := envPlaceholderExprRe.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return input, nil
	}

	var b strings.Builder
	lastEnd := 0
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}

		start, end := m[0], m[1]
		exprStart, exprEnd := m[2], m[3]
		b.WriteString(input[lastEnd:start])

		expr := input[exprStart:exprEnd]
		value, err := resolveEnvExpression(expr)
		if err != nil {
			return "", err
		}
		b.WriteString(value)
		lastEnd = end
	}

	b.WriteString(input[lastEnd:])
	return b.String(), nil
}

func resolveEnvExpression(expr string) (string, error) {
	key := expr
	defaultValue := ""
	hasDefault := false

	if i := strings.Index(expr, ":-"); i >= 0 {
		key = expr[:i]
		defaultValue = expr[i+2:]
		hasDefault = true
	}

	key = strings.TrimSpace(key)
	if !envNameRe.MatchString(key) {
		return "", fmt.Errorf("invalid env placeholder ${%s}", expr)
	}

	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value, nil
	}

	if hasDefault {
		return defaultValue, nil
	}

	return "", fmt.Errorf("unresolved env variable %s", key)
}
