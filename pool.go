package dockertest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type pool struct {
	c          context.Context
	cli        *client.Client
	cancel     context.CancelFunc
	containers []*Container
}

func NewPool(cli *client.Client) (p *pool) {
	c, cancel := context.WithCancel(context.Background())
	p = &pool{
		c:      c,
		cli:    cli,
		cancel: cancel,
	}
	return p
}

func (p *pool) Add(ct *Container) {
	p.containers = append(p.containers, ct)
	return
}

func (p *pool) PullIfNotExist() (err error) {
	for _, ct := range p.containers {
		var out io.ReadCloser
		if out, err = ct.PullIfNotExist(p.c); err != nil {
			err = errors.WithMessagef(err, "Pull Image: %s", ct.Image)
			return
		}
		if out == nil {
			return
		}
		fmt.Printf("%s: 正在拉取镜像...\n", ct.Image)
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			var s = struct {
				Status   string
				Progress string
			}{}
			json.Unmarshal([]byte(scanner.Text()), &s)
			if s.Progress != "" {
				fmt.Printf("%s: Status: %s Progress: %v\n", ct.Image, s.Status, s.Progress)
			} else {
				fmt.Printf("%s: Status: %s\n", ct.Image, s.Status)
			}
		}
		if err = scanner.Err(); err != nil {
			fmt.Printf("%s pull scanner err: %v\n", ct.Image, err)
		}
	}
	return
}

func (p *pool) StartNotRunning() (err error) {
	g := errgroup.Group{}
	for _, ct := range p.containers {
		ct := ct
		if ok, _ := ct.Running(p.c); !ok {
			g.Go(func() error {
				return p.startContainer(ct)
			})
		}
	}
	err = g.Wait()
	return
}

func (p *pool) WaitHealthy() (err error) {
	cs := map[string]int64{}
	for {
		if p.c.Err() != nil {
			return p.c.Err()
		}
		var healthy = true
		for _, c := range p.containers {
			ok, _ := c.Healthy(context.Background())
			if !ok {
				if cs[c.Image] == 0 {
					cs[c.Image] = time.Now().Unix()
				}
				if time.Now().Unix()-cs[c.Image] > 10 {
					cs[c.Image] = time.Now().Unix()
					log.Infof("%s  healthy check: unhealthy", c.shortRef())
				}
				healthy = false
				break
			}
		}
		if !healthy {
			time.Sleep(time.Second)
			continue
		}
		log.Warn("所有镜像启动完毕")
		return
	}
}

func (p *pool) Close() {
	p.cancel()
}

func (p *pool) Purge() {
	for _, ct := range p.containers {
		if ok, _ := ct.Running(context.Background()); !ok {
			continue
		}
		if err := ct.Remove(context.Background()); err != nil {
			log.Infof("%s Remove err: %v", ct.shortRef(), err)
		}
		log.Infof("成功移除%s容器", ct.shortRef())
	}
}

func (p *pool) startContainer(ct *Container) (err error) {
	c := p.c
	image := ct.shortRef()
	if ok, _ := ct.Exist(c); !ok {
		if _, err = ct.Create(c); err != nil {
			err = errors.Wrapf(err, "image: %s", image)
			return
		}
	}
	if err = ct.Start(c); err != nil {
		err = errors.Wrapf(err, "image: %s", image)
		return
	}
	ct.Fresh = true
	go func() {
		rw, err := ct.Logs(c, true)
		if err != nil {
			err = errors.Wrapf(err, "image: %s", image)
			return
		}
		scanner := bufio.NewScanner(rw)
		for scanner.Scan() {
			text := scanner.Text()
			if len(text) > 8 {
				text = text[8:]
			}
			fmt.Printf("%s: %s\n", image, text)
		}
		if err := scanner.Err(); err != nil {
			if err.Error() != "context canceled" {
				fmt.Printf("%s scanner err: %v\n", image, err)
			}
		}
	}()
	return
}

func (p *pool) RunHooks() (err error) {
	for _, ct := range p.containers {
		if len(ct.Hooks) == 0 {
			continue
		}
		for _, hook := range ct.Hooks {
			if len(hook.Cmd) > 0 {
				log.Infof("%s run hook cmd", ct.Image)
				if err := ct.Exec(p.c, hook.Cmd); err != nil {
					return errors.WithStack(err)
				}
			}
			if len(hook.Custom) > 0 {
				if _hooks[hook.Custom] == nil {
					err = errors.Errorf("can't find custom hook: %s", hook.Custom)
					return
				}
				log.Infof("%s run hook %s", ct.Image, hook.Custom)
				if err = _hooks[hook.Custom](ct); err != nil {
					err = errors.Wrapf(err, "run custom hook %v err: %w", hook.Custom, err)
				}
			}
		}
	}
	return
}

func (p *pool) Start() (err error) {
	if err = p.PullIfNotExist(); err != nil {
		return
	}
	if err = p.StartNotRunning(); err != nil {
		return
	}
	if err = p.WaitHealthy(); err != nil {
		return
	}
	err = p.RunHooks()
	return
}
