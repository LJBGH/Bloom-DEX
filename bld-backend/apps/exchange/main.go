package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"bld-backend/apps/exchange/internal/config"
	"bld-backend/apps/exchange/internal/worker"
	"bld-backend/core/util/etcdreg"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

func main() {
	configFile := flag.String("f", "", "config file")
	flag.Parse()

	if *configFile == "" {
		if _, file, _, ok := runtime.Caller(0); ok {
			dir := filepath.Dir(file)
			*configFile = filepath.Join(dir, "etc", "exchange.yaml")
		}
	}

	var c config.Config
	conf.MustLoad(*configFile, &c)
	logx.Must(logx.SetUp(c.Log))

	// 创建一个上下文，用于取消工作
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 如果配置了 etcd，则注册服务
	if len(c.Etcd.Hosts) > 0 {
		reg, err := etcdreg.New(c.Etcd, fmt.Sprintf("%s@%s", c.Name, strconv.Itoa(os.Getpid())))
		if err != nil {
			logx.Must(err)
		}
		if err := reg.Start(ctx); err != nil {
			logx.Must(err)
		}
		defer reg.Stop(context.Background())
	}

	// 启动一个 goroutine，用于监听信号并取消上下文
	go func() {
		// 创建一个通道，用于接收信号
		quit := make(chan os.Signal, 1)
		// 监听信号，SIGINT 和 SIGTERM
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		// 等待信号，阻塞等待信号
		<-quit
		// 取消上下文
		cancel()
	}()

	// 启动工作
	if err := worker.Run(ctx, c); err != nil && err != context.Canceled {
		logx.Errorf("exchange stopped: %v", err)
		os.Exit(1)
	}
	logx.Info("exchange stopped")
}
