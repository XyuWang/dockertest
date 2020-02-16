package dockertest

import (
	"time"

	"gopkg.in/yaml.v2"
)

type YamlConfig struct {
	Version  string               `yaml:"version"`
	Services map[string]*ImageCfg `yaml:"services"`
}

type ImageCfg struct {
	Image       string        `yaml:"image"`
	Ports       []string      `yaml:"ports"`
	Environment []string      `yaml:"environment"`
	Command     []string      `yaml:"command"`
	Volumes     []string      `yaml:"volumes"`
	HealthCheck *HealthyCheck `yaml:"healthcheck"`
	Hooks       []*Hooks      `yaml:"hooks"`
}

type HealthyCheck struct {
	Test     []string      `yaml:"test"`
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
	Retries  int           `yaml:"retries"`
}

type Hooks struct {
	// 通过exec执行容器命令
	Cmd []string `yaml:"cmd"`
	// 使用自定义命令
	Custom string `yaml:"custom"`
}

func DecodeConfig(text []byte) (cfg *YamlConfig, err error) {
	cfg = &YamlConfig{}
	err = yaml.Unmarshal(text, cfg)
	return
}
