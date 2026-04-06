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
	var configFile = flag.String("f", "", "config file")
	flag.Parse()

	if *configFile == "" {
		_, file, _, ok := runtime.Caller(0)
		if ok {
			dir := filepath.Dir(file)
			*configFile = filepath.Join(dir, "etc", "chain-evm-watcher.yaml")
		}
	}

	var c config.Config
	conf.MustLoad(*configFile, &c)
	logx.Must(logx.SetUp(c.Log))

	reg, err := etcdreg.New(c.Etcd, fmt.Sprintf("%s@%d", c.Name, os.Getpid()))
	if err != nil {
		logx.Must(err)
	}
	if err := reg.Start(context.Background()); err != nil {
		logx.Must(err)
	}

	w := worker.New(c)
	w.Start()
	logx.Infof("%s started", c.Name)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	w.Stop()
	reg.Stop(context.Background())
	logx.Infof("%s stopped", c.Name)
}
