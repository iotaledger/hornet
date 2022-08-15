package framework

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// newDockerClient creates a Docker client that communicates via the Docker socket.
func newDockerClient() (*client.Client, error) {
	return client.NewClient(
		"unix:///var/run/docker.sock",
		"",
		nil,
		nil,
	)
}

// DockerContainer is a wrapper object for a Docker container.
type DockerContainer struct {
	client *client.Client
	id     string
}

// NewDockerContainer creates a new DockerContainer.
func NewDockerContainer(c *client.Client) *DockerContainer {
	return &DockerContainer{client: c}
}

// NewDockerContainerFromExisting creates a new DockerContainer from an already existing Docker container by name.
func NewDockerContainerFromExisting(c *client.Client, name string) (*DockerContainer, error) {
	containers, err := c.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	for _, cont := range containers {
		if cont.Names[0] == name {
			return &DockerContainer{
				client: c,
				id:     cont.ID,
			}, nil
		}
	}

	return nil, fmt.Errorf("could not find container with name '%s'", name)
}

// CreateNodeContainer creates a new node container.
func (d *DockerContainer) CreateNodeContainer(cfg *AppConfig) error {
	containerConfig := &container.Config{
		Image:        containerNodeImage,
		ExposedPorts: cfg.ExposedPorts,
		Env:          cfg.Envs,
		Cmd:          cfg.CLIFlags(),
	}

	return d.CreateContainer(cfg.Name, containerConfig, &container.HostConfig{
		Binds: cfg.Binds,
	})
}

// CreateCoordinatorContainer creates a new coordinator container.
func (d *DockerContainer) CreateCoordinatorContainer(cfg *INXCoordinatorConfig) error {
	containerConfig := &container.Config{
		Image: coordinatorImage,
		Env:   cfg.Envs,
		Cmd:   cfg.CLIFlags(),
	}

	return d.CreateContainer(cfg.Name, containerConfig, &container.HostConfig{
		Binds: cfg.Binds,
	})
}

// CreateIndexerContainer creates a new indexer container.
func (d *DockerContainer) CreateIndexerContainer(cfg *INXIndexerConfig) error {
	containerConfig := &container.Config{
		Image: indexerImage,
		Env:   cfg.Envs,
		Cmd:   cfg.CLIFlags(),
	}

	return d.CreateContainer(cfg.Name, containerConfig, &container.HostConfig{
		Binds: cfg.Binds,
	})
}

// CreateWhiteFlagMockContainer creates a new white-flag mock container.
func (d *DockerContainer) CreateWhiteFlagMockContainer(cfg *WhiteFlagMockServerConfig) error {
	containerConfig := &container.Config{
		Image: containerWhiteFlagMockServer,
		ExposedPorts: nat.PortSet{
			"14265/tcp": {},
		},
		Env: cfg.Envs,
	}

	hostCfg := &container.HostConfig{Binds: cfg.Binds}

	return d.CreateContainer(cfg.Name, containerConfig, hostCfg)
}

// CreateContainer creates a new container with the given configuration.
func (d *DockerContainer) CreateContainer(name string, containerConfig *container.Config, hostConfigs ...*container.HostConfig) error {
	var hostConfig *container.HostConfig
	if len(hostConfigs) > 0 {
		hostConfig = hostConfigs[0]
	}

	resp, err := d.client.ContainerCreate(context.Background(), containerConfig, hostConfig, nil, nil, name)
	if err != nil {
		return err
	}

	d.id = resp.ID

	return nil
}

// ConnectToNetwork connects a container to an existent network in the docker host.
func (d *DockerContainer) ConnectToNetwork(networkID string) error {
	return d.client.NetworkConnect(context.Background(), networkID, d.id, nil)
}

// DisconnectFromNetwork disconnects a container from an existent network in the docker host.
func (d *DockerContainer) DisconnectFromNetwork(networkID string) error {
	return d.client.NetworkDisconnect(context.Background(), networkID, d.id, true)
}

// Start sends a request to the docker daemon to start a container.
func (d *DockerContainer) Start() error {
	return d.client.ContainerStart(context.Background(), d.id, types.ContainerStartOptions{})
}

// Remove kills and removes a container from the docker host.
func (d *DockerContainer) Remove() error {
	return d.client.ContainerRemove(context.Background(), d.id, types.ContainerRemoveOptions{Force: true})
}

// Stop stops a container without terminating the process.
// The process is blocked until the container stops or the timeout expires.
func (d *DockerContainer) Stop(optionalTimeout ...time.Duration) error {
	duration := 3 * time.Minute
	if optionalTimeout != nil {
		duration = optionalTimeout[0]
	}

	return d.client.ContainerStop(context.Background(), d.id, &duration)
}

// ExitStatus returns the exit status according to the container information.
func (d *DockerContainer) ExitStatus() (int, error) {
	resp, err := d.client.ContainerInspect(context.Background(), d.id)
	if err != nil {
		return -1, err
	}

	return resp.State.ExitCode, nil
}

// IP returns the IP address according to the container information for the given network.
func (d *DockerContainer) IP(network string) (string, error) {
	resp, err := d.client.ContainerInspect(context.Background(), d.id)
	if err != nil {
		return "", err
	}

	for name, v := range resp.NetworkSettings.Networks {
		if name == network {
			return v.IPAddress, nil
		}
	}

	return "", fmt.Errorf("IP address in %s could not be determined", network)
}

// Logs returns the logs of the container as io.ReadCloser.
func (d *DockerContainer) Logs() (io.ReadCloser, error) {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      "",
		Timestamps: false,
		Follow:     false,
		Tail:       "",
		Details:    false,
	}

	return d.client.ContainerLogs(context.Background(), d.id, options)
}
