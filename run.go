package dockertest

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/docker/docker/client"
)

func Run(configPath string) {
	cli, err := client.NewEnvClient()
	if err != nil {
		fmt.Printf("连接docker失败 请先启动docker\n")
		os.Exit(-1)
	}
	pool := NewPool(cli)
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		panic(err)
	}
	cfg, err := DecodeConfig(data)
	if err != nil {
		panic(err)
	}
	for name, v := range cfg.Services {
		if ct, err := NewContainer(cli, name, v); err != nil {
			fmt.Printf("start Image: %v error: %+v", name, err)
			os.Exit(-1)
		} else {
			pool.Add(ct)
		}
	}
	if err = pool.Start(); err != nil {
		fmt.Printf("start pool fail err: %+v", err)
		os.Exit(-1)
	}
}
