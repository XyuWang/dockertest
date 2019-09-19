package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	dc "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

type pool struct {
	c          context.Context
	cli        *client.Client
	wg         *sync.WaitGroup
	cancel     context.CancelFunc
	containers []*poolContainer
}

type Container struct {
	Image     string
	Port      string
	Mounts    []string
	Env       []string
	Cmd       []string
	CleanCmd  []string
	CleanFunc func()
	InitFunc  func()
	Health    *dc.HealthConfig
	Name      string
}

type poolContainer struct {
	*container
	cleanCmd  []string
	cleanFunc func()
	initFunc  func()
	fresh     bool
}

func NewPool(cli *client.Client) (p *pool) {
	c, cancel := context.WithCancel(context.Background())
	p = &pool{
		c:      c,
		cli:    cli,
		cancel: cancel,
		wg:     &sync.WaitGroup{},
	}
	return p
}

func (p *pool) Add(ct Container) {
	nct := NewContainer(p.cli, ct.Name, ct.Image, ct.Port, ct.Mounts, ct.Env, ct.Cmd, ct.Health)
	p.containers = append(p.containers, &poolContainer{
		container: nct,
		cleanCmd:  ct.CleanCmd,
		cleanFunc: ct.CleanFunc,
		initFunc:  ct.InitFunc,
	})
	return
}

func (p *pool) PullIfNotExist() (err error) {
	for _, ct := range p.containers {
		var out io.ReadCloser
		if out, err = ct.PullIfNotExist(p.c); err != nil {
			log.Infof("%s Pull err:%v", ct.image, err)
			return
		}
		if out == nil {
			return
		}
		log.Infof("%s: 正在拉取镜像...\n", ct.image)
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			var s = struct {
				Status   string
				Progress string
			}{}
			json.Unmarshal([]byte(scanner.Text()), &s)
			if s.Progress != "" {
				log.Infof("%s: Status: %s Progress: %v\n", ct.image, s.Status, s.Progress)
			} else {
				log.Infof("%s: Status: %s\n", ct.image, s.Status)
			}
		}
		if err = scanner.Err(); err != nil {
			log.Infof("%s pull scanner err: %v", ct.image, err)
		}
	}
	return
}

func (p *pool) Start() {
	for _, ct := range p.containers {
		p.wg.Add(1)
		go p.startContainer(ct)
	}
}

func (p *pool) StartNotRunning() {
	for _, ct := range p.containers {
		if ok, _ := ct.Running(p.c); !ok {
			p.wg.Add(1)
			go p.startContainer(ct)
		}
	}
}

func (p *pool) WaitHealthy() {
	cs := map[string]int64{}
	for {
		if p.c.Err() != nil {
			return
		}
		var healthy = true
		for _, c := range p.containers {
			ok, _ := c.Healthy(context.Background())
			if !ok {
				if cs[c.image] == 0 {
					cs[c.image] = time.Now().Unix()
				}
				if time.Now().Unix()-cs[c.image] > 10 {
					cs[c.image] = time.Now().Unix()
					log.Infof("%s  healthy check: unhealthy", c.tag)
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
	p.wg.Wait()
}

func (p *pool) Purge() {
	for _, ct := range p.containers {
		if ok, _ := ct.Running(context.Background()); !ok {
			continue
		}
		if err := ct.Remove(context.Background()); err != nil {
			log.Infof("%s Remove err: %v", ct.tag, err)
		}
		log.Infof("成功移除%s容器", ct.tag)
	}
}

func (p *pool) startContainer(ct *poolContainer) (err error) {
	c, wg := p.c, p.wg
	defer wg.Done()
	image := ct.tag
	if ok, _ := ct.Exist(c); !ok {
		if _, err = ct.Create(c); err != nil {
			log.Infof("%s Create err:%v", image, err)
			return
		}
	}
	if err = ct.Start(c); err != nil {
		log.Infof("%s Start err:%v", image, err)
		return
	}
	ct.fresh = true
	go func() {
		rw, err := ct.Logs(c, true)
		if err != nil {
			log.Infof("%s Logs err:%v", image, err)
			return
		}
		scanner := bufio.NewScanner(rw)
		for scanner.Scan() {
			text := scanner.Text()
			if len(text) > 8 {
				text = text[8:]
			}
			log.Infof("%s: %s", image, text)
		}
		if err := scanner.Err(); err != nil {
			if err.Error() != "context canceled" {
				log.Infof("%s scanner err: %v", image, err)
			}
		}
	}()
	_, _ = ct.Wait(c)
	return
}

func (p *pool) CleanData() (err error) {
	for _, ct := range p.containers {
		if len(ct.cleanCmd) == 0 && ct.cleanFunc == nil {
			continue
		}
		if ct.fresh {
			continue
		}
		if len(ct.cleanCmd) > 0 {
			if err := ct.Exec(p.c, ct.cleanCmd); err != nil {
				log.Errorf("CleanData err: %v", err)
				continue
			}
		}
		if ct.cleanFunc != nil {
			ct.cleanFunc()
		}
		log.Infof("成功清理%s容器数据", ct.tag)
	}
	return
}

func (p *pool) InitData() (err error) {
	for _, ct := range p.containers {
		if ct.initFunc == nil {
			continue
		}
		ct.initFunc()
		log.Infof("成功初始化%s容器数据", ct.tag)
	}
	return
}
