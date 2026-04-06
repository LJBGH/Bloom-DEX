package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	walletpb "bld-backend/api/wallet"
	"bld-backend/apps/walletapi/internal/config"
	"bld-backend/apps/walletapi/internal/handler"
	"bld-backend/apps/walletapi/internal/svc"
	"bld-backend/apps/walletapi/internal/walletrpc"

	"bld-backend/core/util/etcdreg"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

var configFile = flag.String("f", "", "the config file")

func main() {
	flag.Parse()

	if *configFile == "" {
		_, file, _, ok := runtime.Caller(0)
		if ok {
			dir := filepath.Dir(file)
			*configFile = filepath.Join(dir, "etc", "walletapi.yaml")
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

	// 创建服务上下文
	ctxSvc := svc.NewServiceContext(c)

	// 创建 gRPC 服务
	if addr := strings.TrimSpace(c.Rpc.ListenOn); addr != "" {
		rs := zrpc.MustNewServer(c.Rpc, func(grpcServer *grpc.Server) {
			walletpb.RegisterWalletServer(grpcServer, walletrpc.NewServer(ctxSvc))
		})
		go rs.Start()
		fmt.Printf("Starting wallet gRPC (zrpc) at %s...\n", addr)
	}

	// 创建 rest 服务
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	// 注册 handlers
	handler.RegisterHandlers(server, ctxSvc)

	fmt.Printf("Starting rest server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
