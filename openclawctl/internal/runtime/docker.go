package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type DockerRuntime struct{}

type Status struct {
	ContainerName string `json:"containerName"`
	Running       bool   `json:"running"`
	Status        string `json:"status"`
	Health        string `json:"health"`
	Image         string `json:"image"`
}

func NewDockerRuntime() *DockerRuntime {
	return &DockerRuntime{}
}

func (d *DockerRuntime) UpDev(ctx context.Context, composeFile string, env map[string]string) error {
	args := []string{"compose", "-f", resolvePathWithParents(composeFile, 5), "up", "-d", "--build"}
	if out, err := d.run(ctx, env, args...); err != nil {
		return fmt.Errorf("docker compose up: %w (%s)", err, strings.TrimSpace(out))
	}
	return nil
}

func (d *DockerRuntime) DownDev(ctx context.Context, composeFile string, env map[string]string) error {
	args := []string{"compose", "-f", resolvePathWithParents(composeFile, 5), "down"}
	if out, err := d.run(ctx, env, args...); err != nil {
		return fmt.Errorf("docker compose down: %w (%s)", err, strings.TrimSpace(out))
	}
	return nil
}

func (d *DockerRuntime) UpProd(ctx context.Context, params ProdRunParams) error {
	_ = d.DownProd(ctx, params.ContainerName)

	hostConfigDir := filepath.Dir(params.HostConfigPath)
	containerConfigDir := filepath.Dir(params.ContainerConfigPath)
	if err := os.MkdirAll(hostConfigDir, 0o700); err != nil {
		return fmt.Errorf("mkdir host config dir %s: %w", hostConfigDir, err)
	}

	args := []string{
		"run", "-d",
		"--name", params.ContainerName,
		"--restart", "unless-stopped",
		"--init",
		"-p", fmt.Sprintf("127.0.0.1:%d:%d", params.Port, params.Port),
		"-e", "OPENCLAW_CONFIG_PATH=" + params.ContainerConfigPath,
		"-e", "OPENCLAW_STATE_DIR=" + params.ContainerStateDir,
		"-e", "OPENCLAW_GATEWAY_TOKEN=" + params.Token,
		"-v", hostConfigDir + ":" + containerConfigDir,
		"-v", params.StateVolume + ":" + params.ContainerStateDir,
	}
	if len(params.ExtraEnv) > 0 {
		for key, value := range params.ExtraEnv {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				continue
			}
			args = append(args, "-e", key+"="+value)
		}
	}
	args = append(args, params.Image)

	if out, err := d.run(ctx, nil, args...); err != nil {
		return fmt.Errorf("docker run: %w (%s)", err, strings.TrimSpace(out))
	}
	return nil
}

func (d *DockerRuntime) DownProd(ctx context.Context, containerName string) error {
	if containerName == "" {
		containerName = "openclaw-gateway"
	}
	_, err := d.run(ctx, nil, "rm", "-f", containerName)
	if err != nil {
		if strings.Contains(err.Error(), "No such container") {
			return nil
		}
		return err
	}
	return nil
}

func (d *DockerRuntime) Logs(ctx context.Context, containerName string, follow bool) error {
	if containerName == "" {
		containerName = "openclaw-gateway"
	}
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, containerName)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker logs: %w", err)
	}
	return nil
}

func (d *DockerRuntime) Status(ctx context.Context, containerName string) (Status, error) {
	if containerName == "" {
		containerName = "openclaw-gateway"
	}
	out, err := d.run(ctx, nil, "inspect", containerName)
	if err != nil {
		if strings.Contains(out, "No such object") || strings.Contains(out, "No such container") {
			return Status{ContainerName: containerName, Running: false, Status: "not-found"}, nil
		}
		return Status{}, fmt.Errorf("docker inspect: %w (%s)", err, strings.TrimSpace(out))
	}

	var inspected []struct {
		Config struct {
			Image string `json:"Image"`
		} `json:"Config"`
		State struct {
			Running bool   `json:"Running"`
			Status  string `json:"Status"`
			Health  struct {
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
	}

	if err := json.Unmarshal([]byte(out), &inspected); err != nil {
		return Status{}, fmt.Errorf("decode docker inspect: %w", err)
	}
	if len(inspected) == 0 {
		return Status{ContainerName: containerName, Running: false, Status: "not-found"}, nil
	}
	item := inspected[0]

	health := item.State.Health.Status
	if health == "" {
		health = "unknown"
	}
	return Status{
		ContainerName: containerName,
		Running:       item.State.Running,
		Status:        item.State.Status,
		Health:        health,
		Image:         item.Config.Image,
	}, nil
}

func (d *DockerRuntime) run(ctx context.Context, extraEnv map[string]string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	if len(extraEnv) > 0 {
		env := append([]string{}, os.Environ()...)
		for key, value := range extraEnv {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				continue
			}
			env = append(env, key+"="+value)
		}
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

type ProdRunParams struct {
	Image               string
	ContainerName       string
	HostConfigPath      string
	ContainerConfigPath string
	ContainerStateDir   string
	StateVolume         string
	Port                int
	Token               string
	ExtraEnv            map[string]string
}

func ParsePort(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q", raw)
	}
	if v <= 0 || v > 65535 {
		return 0, fmt.Errorf("port out of range: %d", v)
	}
	return v, nil
}

func resolvePathWithParents(path string, maxParentHops int) string {
	cleaned := filepath.Clean(path)
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
