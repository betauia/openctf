package instances

import (
	"context"
	"fmt"
	"strings"
	"time"
	"uuid"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

// Containers carrying this label are challenge instances; others are never touched.
const (
	challengeLabel   = "challenge"
	challengeIDLabel = "challenge_id"
)

// Docker is a Provider that runs instances as containers on the local Docker daemon.
type Docker struct {
	cli        *client.Client
	hostDomain string
}

// NewDocker connects to the Docker daemon configured by the DOCKER_* environment variables.
// hostDomain is reported to players as the host to connect to.
func NewDocker(hostDomain string) (*Docker, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &Docker{cli: cli, hostDomain: hostDomain}, nil
}

// Create starts a container from image with port published on a free host port,
// pulling the image first if it is not present locally.
func (d *Docker) Create(ctx context.Context, challengeID uuid.UUID, image string, port int) (Instance, error) {
	containerPort, err := network.ParsePort(fmt.Sprintf("%d/tcp", port))
	if err != nil {
		return Instance{}, err
	}
	opts := client.ContainerCreateOptions{
		Config: &container.Config{
			Image:        image,
			Labels:       map[string]string{challengeLabel: "true", challengeIDLabel: challengeID.String()},
			ExposedPorts: network.PortSet{containerPort: {}},
		},
		HostConfig: &container.HostConfig{
			// No HostPort: Docker picks a free one, so several instances of a challenge can run at once.
			PortBindings: network.PortMap{containerPort: {{}}},
		},
	}
	created, err := d.cli.ContainerCreate(ctx, opts)
	if cerrdefs.IsNotFound(err) {
		if err := d.pull(ctx, image); err != nil {
			return Instance{}, err
		}
		created, err = d.cli.ContainerCreate(ctx, opts)
	}
	if err != nil {
		return Instance{}, err
	}
	if _, err := d.cli.ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); err != nil {
		d.cli.ContainerRemove(ctx, created.ID, client.ContainerRemoveOptions{Force: true})
		return Instance{}, err
	}

	res, err := d.cli.ContainerInspect(ctx, created.ID, client.ContainerInspectOptions{})
	if err != nil {
		return Instance{}, err
	}
	inst := fromInspect(res.Container)
	inst.HostIP = d.hostDomain
	inst.ChallengeID = &challengeID
	if res.Container.NetworkSettings != nil {
		if b := res.Container.NetworkSettings.Ports[containerPort]; len(b) > 0 {
			inst.HostPort = b[0].HostPort
		}
	}
	return inst, nil
}

func (d *Docker) pull(ctx context.Context, image string) error {
	resp, err := d.cli.ImagePull(ctx, image, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	return resp.Wait(ctx)
}

// List returns all challenge containers, running or not.
func (d *Docker) List(ctx context.Context) ([]Instance, error) {
	res, err := d.cli.ContainerList(ctx, client.ContainerListOptions{
		Filters: client.Filters{}.Add("label", challengeLabel),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Instance, len(res.Items))
	for i, c := range res.Items {
		out[i] = Instance{
			ID:        shortID(c.ID),
			Image:     c.Image,
			Status:    string(c.State),
			CreatedAt: time.Unix(c.Created, 0).UTC().Format(time.RFC3339),
			Provider:  "docker",
		}
		if len(c.Names) > 0 {
			out[i].Name = strings.TrimPrefix(c.Names[0], "/")
		}
	}
	return out, nil
}

// Get returns the challenge container with the given id.
func (d *Docker) Get(ctx context.Context, id string) (Instance, error) {
	c, err := d.inspectChallenge(ctx, id)
	if err != nil {
		return Instance{}, err
	}
	return fromInspect(c), nil
}

// Delete stops and removes the challenge container with the given id.
func (d *Docker) Delete(ctx context.Context, id string) error {
	c, err := d.inspectChallenge(ctx, id)
	if err != nil {
		return err
	}
	if _, err := d.cli.ContainerStop(ctx, c.ID, client.ContainerStopOptions{}); err != nil {
		return err
	}
	_, err = d.cli.ContainerRemove(ctx, c.ID, client.ContainerRemoveOptions{})
	return err
}

// inspectChallenge inspects id, treating containers without the challenge label as not found.
func (d *Docker) inspectChallenge(ctx context.Context, id string) (container.InspectResponse, error) {
	res, err := d.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if cerrdefs.IsNotFound(err) {
		return res.Container, ErrNotFound
	}
	if err != nil {
		return res.Container, err
	}
	if res.Container.Config == nil || res.Container.Config.Labels[challengeLabel] == "" {
		return res.Container, ErrNotFound
	}
	return res.Container, nil
}

func fromInspect(c container.InspectResponse) Instance {
	inst := Instance{
		ID:        shortID(c.ID),
		Name:      strings.TrimPrefix(c.Name, "/"),
		CreatedAt: c.Created,
		Provider:  "docker",
	}
	if c.Config != nil {
		inst.Image = c.Config.Image
		if id, err := uuid.Parse(c.Config.Labels[challengeIDLabel]); err == nil {
			inst.ChallengeID = &id
		}
	}
	if c.State != nil {
		inst.Status = string(c.State.Status)
	}
	return inst
}

// shortID matches docker-py's Container.short_id.
func shortID(id string) string {
	return id[:min(len(id), 12)]
}
