package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/kangeunchan/openclawctl/internal/runtime"
)

func runDown(args []string) error {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	var composeFile string
	fs.StringVar(&composeFile, "compose-file", "", "compose file path for dev profile")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := expandFlagEnv("--compose-file", &composeFile); err != nil {
		return err
	}

	manifest, err := loadManifest()
	if err != nil {
		return err
	}

	dockerRuntime := runtime.NewDockerRuntime()
	ctx := context.Background()

	switch globalOpts.Profile {
	case "dev":
		if strings.TrimSpace(composeFile) == "" {
			composeFile = manifest.Runtime.ComposeFile
		}
		runtimeEnv, err := ensureRuntimeEnv(manifest, false)
		if err != nil {
			return err
		}
		if err := dockerRuntime.DownDev(ctx, composeFile, runtimeEnv.Values); err != nil {
			return err
		}
		outSuccessf("%s runtime stopped for compose file %s", accent("dev"), accent(composeFile))
		return nil
	case "prod":
		containerName := currentContainerName(manifest)
		if err := dockerRuntime.DownProd(ctx, containerName); err != nil {
			return err
		}
		outSuccessf("%s runtime container stopped: %s", accent("prod"), accent(containerName))
		return nil
	default:
		return fmt.Errorf("unsupported profile %q; expected dev or prod", globalOpts.Profile)
	}
}
