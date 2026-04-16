package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"bld-backend/apps/chain-monitor/internal/config"
	"bld-backend/apps/chain-monitor/internal/worker"
	"bld-backend/core/util/etcdreg"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

func main() {
	// 解析配置文件路径
	var configFile = flag.String("f", "", "config file")
	flag.Parse()

	// 如果未指定配置文件路径，使用默认路径
	if *configFile == "" {
		_, file, _, ok := runtime.Caller(0)
		if ok {
			dir := filepath.Dir(file)
			*configFile = filepath.Join(dir, "etc", "chain-evm-watcher.yaml")
		}
	}

	var c config.Config
	conf.MustLoad(*configFile, &c) //加载配置
	logx.Must(logx.SetUp(c.Log))   //初始化日志

	// 注册服务到etcd
	reg, err := etcdreg.New(c.Etcd, fmt.Sprintf("%s@%d", c.Name, os.Getpid()))
	if err != nil {
		logx.Must(err) // 注册失败则直接退出
	}
	// 启动注册
	if err := reg.Start(context.Background()); err != nil {
		logx.Must(err) // 启动注册失败则直接退出
	}

	// 创建并启动worker
	w := worker.New(c)
	w.Start()
	logx.Infof("%s started", c.Name)

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	w.Stop()
	reg.Stop(context.Background())
	logx.Infof("%s stopped", c.Name)
}
