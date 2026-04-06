// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"runtime"

	"bld-backend/apps/ordersapi/internal/config"
	"bld-backend/apps/ordersapi/internal/handler"
	"bld-backend/apps/ordersapi/internal/svc"
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
			*configFile = filepath.Join(dir, "etc", "ordersapi-api.yaml")
		}
	}

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// rest 服务默认不会自动注册 etcd，这里手动注册以便统一服务发现。
	ctxReg, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 获取服务地址
	addrHost := c.Host
	if addrHost == "" || addrHost == "0.0.0.0" || addrHost == "::" {
		addrHost = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", addrHost, c.Port)

	// 注册 etcd
	reg, err := etcdreg.New(c.Etcd, addr)
	if err == nil && c.Etcd.Key != "" {
		if err := reg.Start(ctxReg); err != nil {
			fmt.Printf("etcd register failed: %v\n", err)
		} else {
			defer reg.Stop(context.Background())
		}
	}

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
