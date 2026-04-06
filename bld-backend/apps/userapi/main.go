package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"runtime"

	"bld-backend/apps/userapi/internal/config"
	"bld-backend/apps/userapi/internal/handler"
	"bld-backend/apps/userapi/internal/svc"

	"bld-backend/core/util/etcdreg"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "", "the config file")

func main() {
	flag.Parse()

	if *configFile == "" {
		_, file, _, ok := runtime.Caller(0)
		if ok {
			dir := filepath.Dir(file)
			*configFile = filepath.Join(dir, "etc", "userapi.yaml")
		}
	}

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// rest 服务默认不会自动注册 etcd，这里手动注册以便统一服务发现。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addrHost := c.Host
	if addrHost == "" || addrHost == "0.0.0.0" || addrHost == "::" {
		addrHost = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", addrHost, c.Port)

	// 注册服务：尽量不要影响启动：注册失败只打印日志不致命
	reg, err := etcdreg.New(c.Etcd, addr)
	if err == nil && c.Etcd.Key != "" {
		if err := reg.Start(ctx); err != nil {
			fmt.Printf("etcd register failed: %v\n", err)
		} else {
			defer reg.Stop(context.Background())
		}
	}

	ctxSvc := svc.NewServiceContext(c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	handler.RegisterHandlers(server, ctxSvc)

	fmt.Printf("Starting rest server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
