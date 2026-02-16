package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/kangeunchan/openclawctl/internal/render"
	"github.com/kangeunchan/openclawctl/internal/runtime"
)

func runDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	var against string
	var currentFile string
	fs.StringVar(&against, "against", "live", "diff target: live|file")
	fs.StringVar(&currentFile, "current-file", "", "current config file path when --against=file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := expandFlagEnv("--against", &against); err != nil {
		return err
	}
	if err := expandFlagEnv("--current-file", &currentFile); err != nil {
		return err
	}

	manifest, err := loadManifest()
	if err != nil {
		return err
	}

	_, desiredConfig, err := renderManifest(manifest)
	if err != nil {
		return err
	}
	desiredBytes, err := render.PrettyJSONFromAny(desiredConfig)
	if err != nil {
		return err
	}

	var currentBytes []byte
	switch against {
	case "live":
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client := gatewayClient()
		liveConfig, err := client.GetConfig(ctx)
		if err != nil {
			if strings.TrimSpace(currentFile) == "" {
				currentFile = resolveRuntimeConfigPath(manifest)
			}
			fileBytes, fileErr := runtime.ReadConfigFile(currentFile)
			if fileErr != nil {
				return fmt.Errorf("fetch live config: %w (file fallback failed: %v)", err, fileErr)
			}
			currentBytes, err = normalizeDiffConfig(fileBytes, desiredConfig)
			if err != nil {
				return err
			}
			outWarnf("Live RPC unavailable (%v); using file fallback: %s", err, accent(currentFile))
			break
		}
		currentBytes, err = render.PrettyJSONFromAny(trimToDesiredShape(liveConfig, desiredConfig))
		if err != nil {
			return err
		}
	case "file":
		if strings.TrimSpace(currentFile) == "" {
			currentFile = resolveRuntimeConfigPath(manifest)
		}
		fileBytes, readErr := runtime.ReadConfigFile(currentFile)
		if readErr != nil {
			return readErr
		}
		currentBytes, err = normalizeDiffConfig(fileBytes, desiredConfig)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported --against value: %s", against)
	}

	diffText := simpleUnifiedDiff(string(currentBytes), string(desiredBytes))
	if strings.TrimSpace(diffText) == "" {
		outSuccessf("No diff detected")
		return nil
	}
	fmt.Print(colorizeDiff(diffText))
	return nil
}
