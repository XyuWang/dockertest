# dockertest

## 背景
执行单元测试会依赖一些第三方数据源 如mysql mc redis 如果本地安装测试结果会污染原始数据 会导致每次跑的结果不一致

因此我们依赖docker每次启动干净的容器 但是有些docker容器启动非常慢 常常耗费一分钟以上 如mysql 不如让其常驻后台 只是每次重建数据

项目采用扩展docker-compose配置语法的方式指定容器配置

## 安装

`go get -u github.com/XyuWang/dockertest`

## 使用
```golang
  path := "./docker-compose.yml"
  dockertest.Run(path)
```

实例docker-compose配置
```yaml
version: "3.7"

services:
  redis:
    image: redis
    ports:
      - 16379:6379
    hooks:
      - cmd: ["redis-cli", "flushall"]
  memcached:
    image: memcached:1
    ports:
      - 21211:11211
    hooks:
      - cmd: ["/bin/bash", "-c", "echo 'flush_all' > /dev/tcp/127.0.0.1/11211"]
  db:
    image: mysql:5.6
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
```



## 脚本
```
    hooks:
      - cmd: []
      - custom: string
```
容器启动健康检查通过后 会执行用户指定的hook

cmd类型: 根据cmd中的命令会调用docker exec在容器中执行 
costom类型: 用户自定义类型 

内嵌了 refresh_mysql 脚本
### refresh_mysql
提供以下功能:
   1. 清理容器中除 "information_schema" "mysql"  "performance_schema" 的所有数据库
   2. 根据环境变量 MYSQL_INIT_PATH 指定或者从当前目录往上级寻找test或者resource文件夹 读取出sql文件 重建DB
### 开发
`type HookFunc func(*Container) error`

实现HookFunc方法并调用Register注册即可在custom中调用
