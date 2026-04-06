// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"runtime"

	"bld-backend/apps/gatewayapi/internal/config"
	"bld-backend/apps/gatewayapi/internal/handler"
	"bld-backend/apps/gatewayapi/internal/svc"

	"bld-backend/core/util/etcdreg"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "", "the config file")

func main() {
	flag.Parse()

	// 如果未通过 -f 指定，则自动使用当前源码所在目录下的 etc/gatewayapi-api.yaml
	if *configFile == "" {
		_, file, _, ok := runtime.Caller(0)
		if ok {
			dir := filepath.Dir(file)
			*configFile = filepath.Join(dir, "etc", "gatewayapi-api.yaml")
		}
	}

	// 加载配置
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

	// 注册服务
	reg, err := etcdreg.New(c.Etcd, addr)
	if err == nil && c.Etcd.Key != "" {
		// 尽量不要影响启动：注册失败只打印日志不致命
		if err := reg.Start(ctx); err != nil {
			fmt.Printf("etcd register failed: %v\n", err)
		} else {
			defer reg.Stop(context.Background())
		}
	}

	// 创建服务
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	// 注册服务
	serviceCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, serviceCtx)

	// 启动服务
	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
