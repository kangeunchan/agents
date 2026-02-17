package main

import (
	"context"
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kangeunchan/openclawctl/internal/runtime"
)

func runApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	var mode string
	var dryRun bool
	var configPath string
	fs.StringVar(&mode, "mode", "auto", "apply mode: rpc|file|auto")
	fs.BoolVar(&dryRun, "dry-run", false, "render config without applying")
	fs.StringVar(&configPath, "config-path", "", "openclaw.json path for file mode/fallback")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := expandFlagEnv("--mode", &mode); err != nil {
		return err
	}
	if err := expandFlagEnv("--config-path", &configPath); err != nil {
		return err
	}

	manifest, err := loadManifest()
	if err != nil {
		return err
	}

	desiredBytes, desiredConfig, err := renderManifest(manifest)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Print(string(desiredBytes))
		return nil
	}

	if strings.TrimSpace(configPath) == "" {
		configPath = resolveRuntimeConfigPath(manifest)
	}
	client := gatewayClient()

	switch mode {
	case "rpc":
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := applyViaRPC(ctx, client, desiredConfig); err != nil {
			return err
		}
		outSuccessf("Apply completed via %s", accent("rpc"))
		return nil
	case "file":
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := applyViaFile(ctx, configPath, desiredBytes, currentContainerName(manifest)); err != nil {
			return err
		}
		outSuccessf("Apply completed via %s: %s", accent("file"), accent(configPath))
		return nil
	case "auto":
		rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 45*time.Second)
		err := applyViaRPC(rpcCtx, client, desiredConfig)
		rpcCancel()
		if err == nil {
			outSuccessf("Apply completed via %s", accent("rpc"))
			return nil
		}
		if runtime.IsRPCTransportUnsupported(err) {
			outWarnf("RPC apply unavailable on this gateway build, trying file fallback: %v", err)
		} else {
			outWarnf("RPC apply failed, trying file fallback: %v", err)
		}
		fileCtx, fileCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer fileCancel()
		if fileErr := applyViaFile(fileCtx, configPath, desiredBytes, currentContainerName(manifest)); fileErr != nil {
			return fmt.Errorf("rpc error: %v; file fallback error: %w", err, fileErr)
		}
		outSuccessf("Apply completed via file fallback: %s", accent(configPath))
		return nil
	default:
		return fmt.Errorf("unsupported --mode value: %s", mode)
	}
}

func applyViaRPC(ctx context.Context, client *runtime.GatewayClient, desired map[string]any) error {
	liveConfig, err := client.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("get live config before apply: %w", err)
	}

	if err := client.ApplyConfig(ctx, desired); err != nil {
		return fmt.Errorf("rpc apply failed: %w", err)
	}

	if err := waitForGatewayHealthyAfterRPCApply(ctx, client); err != nil {
		rollbackErr := client.ApplyConfig(ctx, liveConfig)
		reloadErr := client.Reload(ctx)
		if rollbackErr != nil || reloadErr != nil {
			return fmt.Errorf("health check failed after rpc apply: %v; rollback errors: apply=%v reload=%v", err, rollbackErr, reloadErr)
		}
		return fmt.Errorf("health check failed after rpc apply, rolled back: %w", err)
	}
	return nil
}

func waitForGatewayHealthyAfterRPCApply(ctx context.Context, client *runtime.GatewayClient) error {
	var lastErr error

	for {
		if err := healthCheck(ctx, client); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("%w (last error: %v)", ctx.Err(), lastErr)
			}
			return ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
	}
}

func applyViaFile(ctx context.Context, configPath string, desired []byte, containerName string) error {
	backupPath, err := runtime.WriteConfigAtomically(configPath, desired)
	if err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	if err := restartContainer(ctx, containerName); err != nil {
		_ = runtime.RestoreBackup(configPath, backupPath)
		_ = runtime.RemoveBackup(backupPath)
		return fmt.Errorf("container restart failed after file apply: %w", err)
	}

	if err := waitForContainerHealthy(ctx, containerName, 45*time.Second); err != nil {
		restoreErr := runtime.RestoreBackup(configPath, backupPath)
		restartErr := restartContainer(ctx, containerName)
		removeErr := runtime.RemoveBackup(backupPath)
		if restoreErr != nil || restartErr != nil || removeErr != nil {
			return fmt.Errorf("health check failed after file apply: %v; rollback errors: restore=%v restart=%v cleanup=%v", err, restoreErr, restartErr, removeErr)
		}
		return fmt.Errorf("health check failed after file apply, rolled back: %w", err)
	}
	if err := runtime.RemoveBackup(backupPath); err != nil {
		outWarnf("Config apply succeeded but failed to remove rollback backup %s: %v", accent(backupPath), err)
	}
	return nil
}

func restartContainer(ctx context.Context, containerName string) error {
	if strings.TrimSpace(containerName) == "" {
		containerName = "openclaw-gateway"
	}
	cmd := exec.CommandContext(ctx, "docker", "restart", containerName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func waitForContainerHealthy(ctx context.Context, containerName string, timeout time.Duration) error {
	if strings.TrimSpace(containerName) == "" {
		containerName = "openclaw-gateway"
	}
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for container %s to become healthy", containerName)
		}

		cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}", containerName)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("docker inspect health failed: %w (%s)", err, strings.TrimSpace(string(out)))
		}

		status := strings.TrimSpace(string(out))
		switch status {
		case "healthy", "running":
			return nil
		case "unhealthy", "exited", "dead":
			return fmt.Errorf("container %s is %s", containerName, status)
		}

		time.Sleep(2 * time.Second)
	}
}
