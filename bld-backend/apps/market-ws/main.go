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

	"bld-backend/apps/market-ws/internal/config"
	"bld-backend/apps/market-ws/internal/worker"
	"bld-backend/core/util/etcdreg"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

func main() {
	var configFile = flag.String("f", "", "config file")
	flag.Parse()

	if *configFile == "" {
		_, file, _, ok := runtime.Caller(0)
		if ok {
			dir := filepath.Dir(file)
			*configFile = filepath.Join(dir, "etc", "market-ws.yaml")
		}
	}

	var c config.Config
	conf.MustLoad(*configFile, &c)
	logx.Must(logx.SetUp(c.Log))

	// 注册到 etcd
	reg, err := etcdreg.New(c.Etcd, fmt.Sprintf("%s@%d", c.Name, os.Getpid()))
	if err != nil {
		logx.Must(err)
	}
	if err := reg.Start(context.Background()); err != nil {
		logx.Must(err)
	}

	// 启动 worker
	w := worker.New(c)
	w.Start()
	logx.Infof("%s started", c.Name)

	// 监听信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	w.Stop()
	reg.Stop(context.Background())
	logx.Infof("%s stopped", c.Name)
}
