package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	DefaultContainerName = "contextify"
	DefaultImage         = "ghcr.io/atakanatali/contextify:latest"
	DefaultPort          = "8420"
	DefaultVolume        = "contextify-data"
)

type Manager struct {
	ContainerName string
	Image         string
	Port          string
	Volume        string
}

type ContainerStatus struct {
	Exists  bool
	Running bool
	Image   string
	Status  string
}

func NewManager(containerName, image, port string) *Manager {
	if containerName == "" {
		containerName = DefaultContainerName
	}
	if image == "" {
		image = DefaultImage
	}
	if port == "" {
		port = DefaultPort
	}
	return &Manager{
		ContainerName: containerName,
		Image:         image,
		Port:          port,
		Volume:        DefaultVolume,
	}
}

func (m *Manager) IsDockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

func (m *Manager) IsDockerRunning() bool {
	_, err := m.dockerExec(context.Background(), "info", "--format", "{{.ID}}")
	return err == nil
}

func (m *Manager) Pull(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", m.Image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *Manager) Run(ctx context.Context) error {
	args := []string{
		"run", "-d",
		"--name", m.ContainerName,
		"-p", m.Port + ":8420",
		"-v", m.Volume + ":/var/lib/postgresql/data",
		"--restart", "unless-stopped",
		m.Image,
	}
	_, err := m.dockerExec(ctx, args...)
	return err
}

func (m *Manager) Start(ctx context.Context) error {
	_, err := m.dockerExec(ctx, "start", m.ContainerName)
	return err
}

func (m *Manager) Stop(ctx context.Context) error {
	_, err := m.dockerExec(ctx, "stop", m.ContainerName)
	return err
}

func (m *Manager) Remove(ctx context.Context) error {
	_, err := m.dockerExec(ctx, "rm", m.ContainerName)
	return err
}

func (m *Manager) Restart(ctx context.Context) error {
	_, err := m.dockerExec(ctx, "restart", m.ContainerName)
	return err
}

func (m *Manager) Status(ctx context.Context) (*ContainerStatus, error) {
	out, err := m.dockerExec(ctx, "inspect", "--format", "{{json .}}", m.ContainerName)
	if err != nil {
		return &ContainerStatus{Exists: false}, nil
	}

	var inspect struct {
		State struct {
			Running bool   `json:"Running"`
			Status  string `json:"Status"`
		} `json:"State"`
		Config struct {
			Image string `json:"Image"`
		} `json:"Config"`
	}
	if err := json.Unmarshal([]byte(out), &inspect); err != nil {
		return &ContainerStatus{Exists: true}, nil
	}

	return &ContainerStatus{
		Exists:  true,
		Running: inspect.State.Running,
		Image:   inspect.Config.Image,
		Status:  inspect.State.Status,
	}, nil
}

func (m *Manager) Logs(ctx context.Context, follow bool, tail string) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if tail != "" {
		args = append(args, "--tail", tail)
	}
	args = append(args, m.ContainerName)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *Manager) VolumeRemove(name string) error {
	_, err := m.dockerExec(context.Background(), "volume", "rm", name)
	return err
}

func (m *Manager) ImageForVersion(version string) string {
	base := strings.Split(m.Image, ":")[0]
	return fmt.Sprintf("%s:%s", base, version)
}

func (m *Manager) dockerExec(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
