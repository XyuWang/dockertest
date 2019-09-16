package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	dc "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

var (
	redis = Container{
		Image:    "docker.io/library/redis:4",
		Port:     "16379:6379",
		Mounts:   nil,
		Env:      nil,
		Cmd:      nil,
		Health:   nil,
		Name:     "bilitest_redis",
		CleanCmd: []string{"redis-cli", "flushall"},
	}
	mc = Container{
		Image:    "docker.io/library/memcached:1",
		Port:     "21211:11211",
		Mounts:   nil,
		Env:      nil,
		Cmd:      nil,
		Health:   nil,
		Name:     "bilitest_mc",
		CleanCmd: []string{"/bin/bash", "-c", "echo 'flush_all' > /dev/tcp/127.0.0.1/11211"},
	}
	mysql = Container{
		Image: "docker.io/library/mariadb:10.1",
		Port:  "13306:3306",
		Env:   []string{"MYSQL_ALLOW_EMPTY_PASSWORD=yes", "TZ=Asia/Shanghai"},
		Cmd:   nil,
		Health: &dc.HealthConfig{
			Test:     []string{"CMD", "mysqladmin", "ping", "-h", "localhost", "--protocol=tcp"},
			Interval: time.Second * 1,
			Timeout:  time.Second * 2,
			Retries:  20,
		},
		Name:      "bilitest_mariadb",
		CleanFunc: func() { cleanMysql("root:@tcp(127.0.0.1:13306)/?multiStatements=true") },
		InitFunc:  func() { initMysql("root:@tcp(127.0.0.1:13306)/?multiStatements=true") },
	}
	_usage = `容器中启动mc redis mysql测试环境

mc(默认): 127.0.0.1:21211
redis(默认): 127.0.0.1:16379
mysql(可选): 127.0.0.1:13306 用户名root 密码空 上级目录存在resource文件夹时运行

Usage:
  cleanut [option] command 

options:
  -d 后台模式 退出程序不清理容器
  -rm 清理后台容器

Example:
  cleanut go test .  启动容器准备环境 执行命令结束后销毁容器
  cleanut -d go test .  启动或清理容器数据 执行命令 结束后容器继续运行
  cleanut -d  启动或清理容器数据 可在goland test配置中设为前置任务
  cleanut -rm  停止并销毁正在运行的容器
`
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println(_usage)
		return
	}
	if args[0] == "-h" {
		fmt.Println(_usage)
		return
	}
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Errorf("连接docker失败 请执行brew install docker进行安装\n")
		panic(err)
	}
	defer cli.Close()
	pool := NewPool(cli)
	defer pool.Close()
	if args[0] == "-rm" {
		pool.Add(redis)
		pool.Add(mc)
		pool.Add(mysql)
		pool.Close()
		pool.Purge()
		return
	}
	pool.Add(redis)
	pool.Add(mc)
	sqlDir := sqlPath()
	if sqlDir != "" {
		log.Warnf("设定初始化sql目录: %s", sqlDir)
		pool.Add(mysql)
	}
	if args[0] == "-d" {
		daemonMode(pool, args[1:])
		return
	}
	if len(args) == 0 {
		log.Fatal("需附加执行的命令")
		return
	}
	foreMode(pool, args)
}

func daemonMode(pool *pool, args []string) {
	if err := pool.PullIfNotExist(); err != nil {
		os.Exit(1)
		return
	}
	pool.StartNotRunning()
	pool.WaitHealthy()
	pool.CleanData()
	pool.InitData()
	// 不需要执行命令 直接退出
	if len(args) == 0 {
		return
	}
	fmt.Printf("\n")
	runCmd(args[0], args[1:]...)
}

func foreMode(pool *pool, args []string) {
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
		s := <-ch
		log.Warnf("get exist signal %v", s)
		fmt.Printf("\n\n\n\n")
		pool.Close()
		pool.Purge()
	}()
	if err := pool.PullIfNotExist(); err != nil {
		os.Exit(1)
		return
	}
	pool.Start()
	pool.WaitHealthy()
	pool.InitData()
	fmt.Printf("\n\n\n\n")
	runCmd(args[0], args[1:]...)
	fmt.Printf("\n\n\n\n")
	pool.Close()
	pool.Purge()
}

func runCmd(commond string, args ...string) error {
	cmd := exec.Command(commond, args...)
	cmd.Env = os.Environ()
	cmd.Dir, _ = os.Getwd()
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
