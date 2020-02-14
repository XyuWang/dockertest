package dockertest

import (
	"reflect"
	"testing"
	"time"
)

func TestDecodeConfig(t *testing.T) {
	var _testCfg = `

version: "3.7"

services:
  db:
    image: mariadb:10.1
    ports:
    - 13306:3306
    environment:
    - MYSQL_ALLOW_EMPTY_PASSWORD=yes
    - TZ=Asia/Shanghai
    command: [
      '--character-set-server=utf8',
      '--collation-server=utf8_unicode_ci'
    ]
    volumes:
      - .:/docker-entrypoint-initdb.d
    Healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost", "--protocol=tcp"]
      interval: 1s
      timeout: 2s
      retries: 20
      start_period: 5s
    hooks:
      - custom: refresh_mysql
  redis:
    image: redis
    ports:
      - 16379:6379
    hooks:
      - cmd: ["redis-cli", "flushall"]
`

	wantCfg := &YamlConfig{
		Version: "3.7",
		Services: map[string]*ImageCfg{
			"redis": {
				Image: "redis",
				Ports: []string{"16379:6379"},
				Hooks: []*Hooks{{
					Cmd: []string{"redis-cli", "flushall"},
				}},
			},
			"db": {
				Image:       "mariadb:10.1",
				Ports:       []string{"13306:3306"},
				Environment: []string{"MYSQL_ALLOW_EMPTY_PASSWORD=yes", "TZ=Asia/Shanghai"},
				Command:     []string{"--character-set-server=utf8", "--collation-server=utf8_unicode_ci"},
				Volumes:     []string{".:/docker-entrypoint-initdb.d"},
				HealthCheck: &HealthyCheck{
					Test:     []string{"CMD", "mysqladmin", "ping", "-h", "localhost", "--protocol=tcp"},
					Interval: time.Second,
					Timeout:  time.Second * 2,
					Retries:  20,
				},
				Hooks: []*Hooks{{
					Custom: "refresh_mysql",
				}},
			},
		},
	}

	type args struct {
		text string
	}
	tests := []struct {
		name    string
		args    args
		wantCfg *YamlConfig
		wantErr bool
	}{
		{
			name:    "test normal",
			args:    args{text: _testCfg},
			wantErr: false,
			wantCfg: wantCfg,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCfg, err := DecodeConfig([]byte(tt.args.text))
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotCfg, tt.wantCfg) {
				t.Errorf("DecodeConfig() gotCfg = %v, want %v", gotCfg, tt.wantCfg)
			}
		})
	}
}
