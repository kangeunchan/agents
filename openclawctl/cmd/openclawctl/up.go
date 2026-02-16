package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kangeunchan/openclawctl/internal/runtime"
)

func runUp(args []string) error {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	var image string
	var composeFile string
	var configPath string
	var stateVolume string
	fs.StringVar(&image, "image", "", "runtime image for prod profile")
	fs.StringVar(&composeFile, "compose-file", "", "compose file path for dev profile")
	fs.StringVar(&configPath, "config-path", "", "host path for rendered openclaw.json")
	fs.StringVar(&stateVolume, "state-volume", "", "docker volume name for runtime state")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := expandFlagEnv("--image", &image); err != nil {
		return err
	}
	if err := expandFlagEnv("--compose-file", &composeFile); err != nil {
		return err
	}
	if err := expandFlagEnv("--config-path", &configPath); err != nil {
		return err
	}
	if err := expandFlagEnv("--state-volume", &stateVolume); err != nil {
		return err
	}

	manifest, err := loadManifest()
	if err != nil {
		return err
	}

	rendered, _, err := renderManifest(manifest)
	if err != nil {
		return err
	}

	if strings.TrimSpace(configPath) == "" {
		configPath = resolveRuntimeConfigPath(manifest)
	}
	if strings.TrimSpace(image) == "" {
		image = manifest.Runtime.Image
	}
	if strings.TrimSpace(composeFile) == "" {
		composeFile = manifest.Runtime.ComposeFile
	}
	if strings.TrimSpace(stateVolume) == "" {
		stateVolume = manifest.Runtime.StateVolume
	}

	if err := writeFile(configPath, rendered); err != nil {
		return err
	}

	dockerRuntime := runtime.NewDockerRuntime()
	ctx := context.Background()
	runtimeEnv, err := ensureRuntimeEnv(manifest, true)
	if err != nil {
		return err
	}

	switch globalOpts.Profile {
	case "dev":
		if err := dockerRuntime.UpDev(ctx, composeFile, runtimeEnv.Values); err != nil {
			return err
		}
		outSuccessf("%s runtime started with compose file %s", accent("dev"), accent(composeFile))
		return nil
	case "prod":
		token := globalOpts.GatewayToken
		if strings.TrimSpace(token) == "" {
			token = os.Getenv(manifest.Gateway.TokenEnv)
		}
		if strings.TrimSpace(token) == "" {
			return fmt.Errorf("gateway token is required for prod; set --gateway-token or env %s", manifest.Gateway.TokenEnv)
		}

		containerName := currentContainerName(manifest)
		oldStatus, _ := dockerRuntime.Status(ctx, containerName)

		absoluteConfigPath, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("resolve absolute config path: %w", err)
		}

		params := runtime.ProdRunParams{
			Image:               image,
			ContainerName:       containerName,
			HostConfigPath:      absoluteConfigPath,
			ContainerConfigPath: "/etc/openclaw-config/openclaw.json",
			ContainerStateDir:   manifest.Runtime.StateDir,
			StateVolume:         stateVolume,
			Port:                manifest.Gateway.Port,
			Token:               token,
			ExtraEnv:            runtimeEnv.Values,
		}

		if err := dockerRuntime.UpProd(ctx, params); err != nil {
			return err
		}

		healthCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := healthCheck(healthCtx, gatewayClient()); err != nil {
			if strings.TrimSpace(oldStatus.Image) != "" && oldStatus.Image != image {
				rollbackParams := params
				rollbackParams.Image = oldStatus.Image
				rollbackErr := dockerRuntime.UpProd(ctx, rollbackParams)
				if rollbackErr != nil {
					return fmt.Errorf("new container unhealthy (%v) and rollback failed: %w", err, rollbackErr)
				}
				return fmt.Errorf("new container unhealthy, rolled back to %s: %w", oldStatus.Image, err)
			}
			return fmt.Errorf("new container unhealthy and no rollback image available: %w", err)
		}
		outSuccessf("%s runtime started with image %s", accent("prod"), accent(image))
		return nil
	default:
		return fmt.Errorf("unsupported profile %q; expected dev or prod", globalOpts.Profile)
	}
}
