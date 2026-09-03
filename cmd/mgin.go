package main

import (
	"fmt"
	"os"
)

// MGin 脚手架命令行工具（模仿 beego 的 bee 命令）
//
// 用法:
//
//	go build -o mgin ./cmd
//	./mgin new <工程名> [选项]
//
// 示例:
//
//	./mgin new myservice --db mysql,redis --mq nats,kafka --port 8080 --registry nacos --config-center nacos
//
// 支持的选项（均可交互输入）：
//
//	工程名 / Go module 路径 / 端口 / 数据库 / 消息队列 / 注册中心 / 配置中心 / JWT / Casbin / i18n
//
// 子命令:
//
//	new       创建一个 mgin 微服务工程骨架
//	version   查看脚手架版本
func main() {
	args := os.Args
	if len(args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch args[1] {
	case "new":
		if err := runNew(args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "创建失败: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	case "version", "-v", "--version":
		fmt.Println("mgin scaffold " + toolVersion)
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`MGin 项目脚手架

用法:
  mgin new <工程名> [选项]

选项:
  --module <path>         Go module 路径 (默认 github.com/maczh/<工程名>)
  --port <port>           HTTP 端口 (默认 8080)
  --db <list>             数据库, 逗号分隔:
                          mysql,postgres,sqlite,mongodb,redis,clickhouse,elasticsearch
                          (默认 mysql)
  --mq <list>             消息队列, 逗号分隔多选:
                          nats,kafka,mqtt,rabbit (默认不使用)
                          分别使用插件 maczh/nats、maczh/mgkafka、maczh/mqtt、maczh/mgrabbit
  --registry <type>       注册中心: nacos,consul,etcd 或 none (默认 none)
  --config-center <type>  配置中心: nacos,consul,etcd,polaris,springconfig,file,none (默认 none)
  --i18n                  启用国际化
  --jwt                   启用 JWT 鉴权中间件
  --casbin                启用 Casbin 接口鉴权
  --sys                   启用内置系统管理模块 (仅 master 分支, jh 分支已移除)
  --output <dir>          输出目录 (默认当前目录)
  --mgin-version <ver>    mgin 依赖版本 (默认自动获取最新发布版)
  --force                 目录已存在时覆盖文件

交互模式: 省略任意选项将在终端中逐项询问；非终端环境自动使用默认值。
`)
}
