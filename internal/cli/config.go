package cli

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultServerURL     = "http://localhost:8420"
	defaultDockerImage   = "ghcr.io/atakanatali/contextify:latest"
	defaultContainerName = "contextify"
	defaultPort          = "8420"
)

type CLIConfig struct {
	ServerURL     string `yaml:"server_url"`
	DockerImage   string `yaml:"docker_image"`
	ContainerName string `yaml:"container_name"`
	Port          string `yaml:"port"`
}

func loadCLIConfig() *CLIConfig {
	cfg := &CLIConfig{
		ServerURL:     defaultServerURL,
		DockerImage:   defaultDockerImage,
		ContainerName: defaultContainerName,
		Port:          defaultPort,
	}

	configPath := flagConfig
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return cfg
		}
		configPath = filepath.Join(home, ".contextify", "config.yaml")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg
	}

	_ = yaml.Unmarshal(data, cfg)

	// Apply defaults for empty fields
	if cfg.ServerURL == "" {
		cfg.ServerURL = defaultServerURL
	}
	if cfg.DockerImage == "" {
		cfg.DockerImage = defaultDockerImage
	}
	if cfg.ContainerName == "" {
		cfg.ContainerName = defaultContainerName
	}
	if cfg.Port == "" {
		cfg.Port = defaultPort
	}

	return cfg
}

func getServerURL() string {
	if flagServerURL != "" {
		return flagServerURL
	}
	if env := os.Getenv("CONTEXTIFY_URL"); env != "" {
		return env
	}
	return loadCLIConfig().ServerURL
}

func getDockerImage() string {
	if env := os.Getenv("CONTEXTIFY_IMAGE"); env != "" {
		return env
	}
	return loadCLIConfig().DockerImage
}

func getContainerName() string {
	if env := os.Getenv("CONTEXTIFY_CONTAINER"); env != "" {
		return env
	}
	return loadCLIConfig().ContainerName
}

func getPort() string {
	if env := os.Getenv("CONTEXTIFY_PORT"); env != "" {
		return env
	}
	return loadCLIConfig().Port
}
