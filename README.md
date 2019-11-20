# cleanut

## 背景
执行单元测试会依赖一些第三方数据源 如mysql mc redis 如果本地安装测试结果会污染原始数据 会导致每次跑的结果不一致

依赖docker是一个可行的办法 但是docker存在启动容器慢的问题 我们希望他能常驻在后台 每次跑UT时候再清理并重建数据 

因此有了这个工具

工具会默认启动三个常用依赖 mc redis mysql 并在每次执行的时候初始化数据

## 安装

`go get -u github.com/XyuWang/cleanut`

## 命令行使用方法

mc(默认): 127.0.0.1:21211

redis(默认): 127.0.0.1:16379

mysql(可选): 127.0.0.1:13306 用户名root 密码空 上级目录存在test或者resource文件夹时运行 按字母序执行*.sql文件初始化db

```bash
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
  ```
## goland配置方式

Preferences > Extenal Tools > Add


Program: cleanut

Arguments: -d

Work Dir: $FileDir$
