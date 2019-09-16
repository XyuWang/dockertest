
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
  
