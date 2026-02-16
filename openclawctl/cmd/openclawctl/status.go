package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kangeunchan/openclawctl/internal/runtime"
	v1 "github.com/kangeunchan/openclawctl/internal/schema/v1"
)

type statusOutput struct {
	CheckedAt string         `json:"checkedAt"`
	Docker    runtime.Status `json:"docker"`
	Gateway   string         `json:"gateway"`
	UI        string         `json:"ui"`
	RPCError  string         `json:"rpcError,omitempty"`
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	var jsonOut bool
	var observe bool
	var intervalRaw string
	fs.BoolVar(&jsonOut, "json", false, "print machine-readable JSON status")
	fs.BoolVar(&observe, "observe", false, "continuously monitor status until Ctrl+C")
	fs.BoolVar(&observe, "o", false, "continuously monitor status until Ctrl+C")
	fs.StringVar(&intervalRaw, "interval", "3s", "observe refresh interval (e.g. 1s, 2s, 5s)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := expandFlagEnv("--interval", &intervalRaw); err != nil {
		return err
	}

	interval, err := time.ParseDuration(strings.TrimSpace(intervalRaw))
	if err != nil {
		return fmt.Errorf("invalid --interval value %q: %w", intervalRaw, err)
	}
	if interval <= 0 {
		return fmt.Errorf("--interval must be greater than 0")
	}

	manifest, err := loadManifest()
	if err != nil {
		return err
	}

	if !observe {
		output, err := collectStatus(manifest)
		if err != nil {
			return err
		}
		return printStatus(output, jsonOut, false, interval)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	interactive := !jsonOut && isTTY(os.Stdout)
	if interactive {
		_, _ = fmt.Fprint(os.Stdout, "\x1b[?25l")
		defer func() {
			_, _ = fmt.Fprint(os.Stdout, "\x1b[?25h")
		}()
	}

	for {
		output, err := collectStatus(manifest)
		if err != nil {
			return err
		}

		if interactive {
			_, _ = fmt.Fprint(os.Stdout, "\x1b[H\x1b[2J")
		}
		if err := printStatus(output, jsonOut, true, interval); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			if !jsonOut {
				outInfof("Observe stopped")
			}
			return nil
		case <-ticker.C:
		}
	}
}

func collectStatus(manifest *v1.Manifest) (statusOutput, error) {
	dockerRuntime := runtime.NewDockerRuntime()
	containerName := currentContainerName(manifest)
	dockerStatus, err := dockerRuntime.Status(context.Background(), containerName)
	if err != nil {
		return statusOutput{}, err
	}

	client := gatewayClient()
	gatewayHealth := "unreachable"
	uiStatus := "unreachable"
	var rpcErr string

	healthCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := client.Health(healthCtx); err == nil {
		gatewayHealth = "healthy"
	} else {
		rpcErr = err.Error()
	}

	uiCtx, uiCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer uiCancel()
	if ok, err := client.UIReachable(uiCtx); err == nil && ok {
		uiStatus = "reachable"
	}

	if gatewayHealth != "healthy" {
		switch {
		case dockerStatus.Health == "healthy" && uiStatus == "reachable":
			gatewayHealth = "healthy (docker/ui; rpc unavailable)"
		case dockerStatus.Health == "healthy":
			gatewayHealth = "healthy (docker)"
		case uiStatus == "reachable":
			gatewayHealth = "ui-reachable"
		}
	}

	return statusOutput{
		CheckedAt: time.Now().Format(time.RFC3339),
		Docker:    dockerStatus,
		Gateway:   gatewayHealth,
		UI:        uiStatus,
		RPCError:  rpcErr,
	}, nil
}

func printStatus(output statusOutput, jsonOut bool, observe bool, interval time.Duration) error {
	if jsonOut {
		var encoded []byte
		var err error
		if observe {
			encoded, err = json.Marshal(output)
		} else {
			encoded, err = json.MarshalIndent(output, "", "  ")
		}
		if err != nil {
			return err
		}
		fmt.Println(string(encoded))
		return nil
	}

	fmt.Printf("%s %s\n", keyLabel("checked at:"), accent(output.CheckedAt))
	fmt.Printf("%s %s\n", keyLabel("container:"), accent(output.Docker.ContainerName))
	fmt.Printf("%s %s\n", keyLabel("running:"), boolBadge(output.Docker.Running))
	fmt.Printf("%s %s\n", keyLabel("state:"), stateBadge(output.Docker.Status))
	fmt.Printf("%s %s\n", keyLabel("docker health:"), stateBadge(output.Docker.Health))
	fmt.Printf("%s %s\n", keyLabel("gateway health:"), stateBadge(output.Gateway))
	fmt.Printf("%s %s\n", keyLabel("ui:"), stateBadge(output.UI))
	if output.RPCError != "" {
		fmt.Printf("%s %s\n", keyLabel("rpc:"), paint(printer.stdout, output.RPCError, ansiFgYellow))
	}
	fmt.Printf("%s %s\n", keyLabel("image:"), accent(output.Docker.Image))
	if observe {
		fmt.Printf("%s %s (%s %s)\n", keyLabel("observe:"), stateBadge("running"), keyLabel("interval"), accent(interval.String()))
		fmt.Printf("%s %s\n", keyLabel("stop:"), paint(printer.stdout, "Ctrl+C", ansiBold, ansiFgYellow))
	}
	return nil
}

func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
