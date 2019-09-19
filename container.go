package main

import (
	"context"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	dc "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type container struct {
	cli          *client.Client
	image        string
	tag          string
	id           string
	portBindings nat.PortMap
	mounts       []mount.Mount
	env          []string
	cmd          []string
	healthcheck  *dc.HealthConfig
	name         string
}

func NewContainer(cli *client.Client, name, image string, portBinding string, mounts, env, cmd []string, healthy *dc.HealthConfig) *container {
	tags := strings.Split(image, "/")
	port := nat.PortMap{}
	if portBinding != "" {
		arr := strings.Split(portBinding, ":")
		port[nat.Port(arr[1]+"/tcp")] = []nat.PortBinding{{
			HostIP:   "127.0.0.1",
			HostPort: arr[0] + "/tcp",
		}}
	}
	var ms []mount.Mount
	for _, m := range mounts {
		as := strings.Split(m, ":")
		if len(as) != 2 {
			continue
		}
		ms = append(ms, mount.Mount{
			Type:   mount.TypeBind,
			Source: as[0],
			Target: as[1],
		})
	}
	return &container{
		cli:          cli,
		image:        image,
		tag:          tags[len(tags)-1],
		portBindings: port,
		mounts:       ms,
		env:          env,
		cmd:          cmd,
		healthcheck:  healthy,
		name:         name,
	}
}

func (ct *container) ID() string {
	if ct.id == "" {
		return ct.name
	}
	return ct.id
}

func (ct *container) Name() string {
	return ct.name
}

func (ct *container) Pull(c context.Context) (io.ReadCloser, error) {
	return ct.cli.ImagePull(c, ct.image, types.ImagePullOptions{})
}

func (ct *container) PullIfNotExist(c context.Context) (io.ReadCloser, error) {
	list, err := ct.cli.ImageList(c, types.ImageListOptions{})
	if err != nil {
		return nil, err
	}
	for _, l := range list {
		for _, t := range l.RepoTags {
			if t == ct.tag {
				return nil, nil
			}
		}
	}
	return ct.Pull(c)
}

func (ct *container) Create(c context.Context) (dc.ContainerCreateCreatedBody, error) {
	host := &dc.HostConfig{
		Binds:        nil,
		LogConfig:    dc.LogConfig{},
		NetworkMode:  "",
		PortBindings: ct.portBindings,
		Mounts:       ct.mounts,
	}
	body, err := ct.cli.ContainerCreate(c, &dc.Config{
		Image:       ct.tag,
		Env:         ct.env,
		Cmd:         ct.cmd,
		Healthcheck: ct.healthcheck,
	}, host, nil, ct.name)
	if err == nil {
		ct.id = body.ID
	}
	return body, err
}

func (ct *container) Start(c context.Context) (err error) {
	return ct.cli.ContainerStart(c, ct.ID(), types.ContainerStartOptions{})
}

func (ct *container) Wait(c context.Context) (int64, error) {
	return ct.cli.ContainerWait(c, ct.ID())
}

func (ct *container) Logs(c context.Context, follow bool) (io.ReadCloser, error) {
	return ct.cli.ContainerLogs(c, ct.ID(), types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Tail:       "",
	})
}

func (ct *container) Kill(c context.Context) error {
	return ct.cli.ContainerKill(c, ct.ID(), "KILL")
}

func (ct *container) Remove(c context.Context) error {
	return ct.cli.ContainerRemove(c, ct.ID(), types.ContainerRemoveOptions{
		RemoveVolumes: true,
		RemoveLinks:   false,
		Force:         true,
	})
}

func (ct *container) SimpleStartAndWait(c context.Context) (err error) {
	if _, err = ct.Create(c); err != nil {
		panic(err)
	}
	if err = ct.Start(c); err != nil {
		panic(err)
	}
	_, err = ct.Wait(c)
	return err
}

func (ct *container) Healthy(c context.Context) (ok bool, err error) {
	cj, err := ct.cli.ContainerInspect(c, ct.ID())
	if err != nil {
		return
	}
	health := cj.ContainerJSONBase.State.Health
	if health == nil {
		return true, nil
	}
	return health.Status == types.Healthy, nil
}

func (ct *container) Running(c context.Context) (ok bool, err error) {
	var status types.ContainerJSON
	if status, err = ct.cli.ContainerInspect(c, ct.name); err != nil {
		if client.IsErrContainerNotFound(err) {
			err = nil
		}
		return
	}
	return status.State.Running, nil
}

func (ct *container) Exist(c context.Context) (ok bool, err error) {
	if _, err = ct.cli.ContainerInspect(c, ct.name); err != nil {
		if client.IsErrContainerNotFound(err) {
			return false, nil
		}
		return
	}
	return true, nil
}

func (ct *container) Exec(c context.Context, cmd []string) (err error) {
	res, err := ct.cli.ContainerExecCreate(c, ct.ID(), types.ExecConfig{
		Cmd: cmd,
	})
	if err != nil {
		return
	}
	err = ct.cli.ContainerExecStart(c, res.ID, types.ExecStartCheck{
		Detach: false,
		Tty:    false,
	})
	return
}
