package main

import (
	"fmt"
	"strings"

	"github.com/kangeunchan/openclawctl/internal/config"
)

func expandFlagEnv(name string, value *string) error {
	if value == nil {
		return nil
	}
	if strings.TrimSpace(*value) == "" {
		return nil
	}
	resolved, err := config.ExpandEnvString(*value)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	*value = resolved
	return nil
}
