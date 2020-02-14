package dockertest

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	dc "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
)

type Container struct {
	cli          *client.Client
	Name         string
	ImageCfg     *ImageCfg
	Image        string
	PortBindings nat.PortMap
	Mounts       []mount.Mount
	Env          []string
	Cmd          []string
	Healthcheck  *dc.HealthConfig
	Hooks        []*Hooks
	// 是否是新启动的镜像
	Fresh bool
}

func NewContainer(cli *client.Client, name string, imageCfg *ImageCfg) (ct *Container, err error) {
	port := nat.PortMap{}
	if len(imageCfg.Ports) > 0 {
		for _, p := range imageCfg.Ports {
			arr := strings.Split(p, ":")
			if len(arr) != 2 {
				err = errors.New(fmt.Sprintf("wrong port: %v", p))
				return
			}
			port[nat.Port(arr[1]+"/tcp")] = []nat.PortBinding{{
				HostIP:   "127.0.0.1",
				HostPort: arr[0] + "/tcp",
			}}
		}
	}
	var ms []mount.Mount
	mounts := imageCfg.Volumes
	for _, m := range mounts {
		as := strings.Split(m, ":")
		if len(as) != 2 {
			err = errors.New(fmt.Sprintf("wrong volumn: %v", m))
			return
		}
		path, err := filepath.Abs(as[0])
		if err != nil {
			return ct, errors.WithStack(err)
		}
		ms = append(ms, mount.Mount{
			Type:   mount.TypeBind,
			Source: path,
			Target: as[1],
		})
	}
	var healthy *dc.HealthConfig
	if imageCfg.HealthCheck != nil {
		healthy = &dc.HealthConfig{
			Test:     imageCfg.HealthCheck.Test,
			Interval: imageCfg.HealthCheck.Interval,
			Timeout:  imageCfg.HealthCheck.Timeout,
			Retries:  imageCfg.HealthCheck.Retries,
		}
	}
	ct = &Container{
		cli:          cli,
		ImageCfg:     imageCfg,
		Image:        imageCfg.Image,
		PortBindings: port,
		Mounts:       ms,
		Env:          imageCfg.Environment,
		Cmd:          imageCfg.Command,
		Healthcheck:  healthy,
		Name:         name,
	}
	return
}

func (ct *Container) shortRef() (t string) {
	ref := ct.Image
	if strings.Contains(ref, ":") {
		return ref
	}
	return ref + ":latest"
}

func (ct *Container) longRef() (t string) {
	ref := ct.Image
	if strings.Contains(ref, "/") {
		return ref
	}
	return "docker.io/library/" + ref
}

func (ct *Container) Pull(c context.Context) (io.ReadCloser, error) {
	return ct.cli.ImagePull(c, ct.longRef(), types.ImagePullOptions{})
}

func (ct *Container) PullIfNotExist(c context.Context) (io.ReadCloser, error) {
	list, err := ct.cli.ImageList(c, types.ImageListOptions{})
	if err != nil {
		return nil, err
	}
	for _, l := range list {
		for _, t := range l.RepoTags {
			if t == ct.shortRef() {
				return nil, nil
			}
		}
	}
	return ct.Pull(c)
}

func (ct *Container) Create(c context.Context) (dc.ContainerCreateCreatedBody, error) {
	host := &dc.HostConfig{
		Binds:        nil,
		LogConfig:    dc.LogConfig{},
		NetworkMode:  "",
		PortBindings: ct.PortBindings,
		Mounts:       ct.Mounts,
	}
	return ct.cli.ContainerCreate(c, &dc.Config{
		Image:       ct.longRef(),
		Env:         ct.Env,
		Cmd:         ct.Cmd,
		Healthcheck: ct.Healthcheck,
	}, host, nil, ct.Name)
}

func (ct *Container) Start(c context.Context) (err error) {
	return ct.cli.ContainerStart(c, ct.Name, types.ContainerStartOptions{})
}

func (ct *Container) Wait(c context.Context) (int64, error) {
	return ct.cli.ContainerWait(c, ct.Name)
}

func (ct *Container) Logs(c context.Context, follow bool) (io.ReadCloser, error) {
	return ct.cli.ContainerLogs(c, ct.Name, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Tail:       "",
	})
}

func (ct *Container) Kill(c context.Context) error {
	return ct.cli.ContainerKill(c, ct.Name, "KILL")
}

func (ct *Container) Remove(c context.Context) error {
	return ct.cli.ContainerRemove(c, ct.Name, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		RemoveLinks:   false,
		Force:         true,
	})
}

func (ct *Container) SimpleStartAndWait(c context.Context) (err error) {
	if _, err = ct.Create(c); err != nil {
		panic(err)
	}
	if err = ct.Start(c); err != nil {
		panic(err)
	}
	_, err = ct.Wait(c)
	return err
}

func (ct *Container) Healthy(c context.Context) (ok bool, err error) {
	cj, err := ct.cli.ContainerInspect(c, ct.Name)
	if err != nil {
		return
	}
	health := cj.ContainerJSONBase.State.Health
	if health == nil {
		return true, nil
	}
	return health.Status == types.Healthy, nil
}

func (ct *Container) Running(c context.Context) (ok bool, err error) {
	var status types.ContainerJSON
	if status, err = ct.cli.ContainerInspect(c, ct.Name); err != nil {
		if client.IsErrContainerNotFound(err) {
			err = nil
		}
		return
	}
	return status.State.Running, nil
}

func (ct *Container) Exist(c context.Context) (ok bool, err error) {
	if _, err = ct.cli.ContainerInspect(c, ct.Name); err != nil {
		if client.IsErrContainerNotFound(err) {
			return false, nil
		}
		return
	}
	return true, nil
}

func (ct *Container) Exec(c context.Context, cmd []string) (err error) {
	res, err := ct.cli.ContainerExecCreate(c, ct.Name, types.ExecConfig{
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
