package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"bld-backend/apps/gateway/internal/config"
	"bld-backend/apps/gateway/internal/middleware"
	"bld-backend/core/util/etcdreg"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/gateway"
)

var (
	baseConfigFile   = flag.String("c", "", "base config file (config.yaml)")
	gatewayRouteFile = flag.String("g", "", "gateway routes file (gateway-config.yaml)")
)

// appConfig 应用配置
type appConfig struct {
	Name      string
	Log       logx.LogConf
	Host      string
	Port      int
	RateLimit config.RateLimitConf
	Redis     redis.RedisConf
	Etcd      etcdreg.Config
}

// routeConfig 路由配置
type routeConfig struct {
	Upstreams []gateway.Upstream
}

func main() {
	flag.Parse()

	// 默认加载两个配置文件
	if *baseConfigFile == "" || *gatewayRouteFile == "" {
		_, file, _, ok := runtime.Caller(0)
		if ok {
			dir := filepath.Dir(file)
			if *baseConfigFile == "" {
				*baseConfigFile = filepath.Join(dir, "etc", "config.yaml")
			}
			if *gatewayRouteFile == "" {
				*gatewayRouteFile = filepath.Join(dir, "etc", "gateway-config.yaml")
			}
		}
	}

	var base appConfig
	conf.MustLoad(*baseConfigFile, &base)
	logx.Must(logx.SetUp(base.Log))

	var routes routeConfig
	conf.MustLoad(*gatewayRouteFile, &routes)
	var gwConf gateway.GatewayConf
	// 合并 base 配置到 gateway 配置
	gwConf.Name = base.Name
	gwConf.Host = base.Host
	gwConf.Port = base.Port
	gwConf.Log = base.Log
	gwConf.Upstreams = routes.Upstreams

	// 创建一个上下文，用于取消工作
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addrHost := base.Host
	if addrHost == "" || addrHost == "0.0.0.0" || addrHost == "::" {
		addrHost = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", addrHost, base.Port)

	// 端口预检查：避免端口占用时仍注册到 etcd
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logx.Errorf("listen %s failed: %v", addr, err)
		return
	}
	_ = ln.Close()

	// 注册到 etcd
	reg, err := etcdreg.New(base.Etcd, addr)
	if err == nil && base.Etcd.Key != "" {
		if err := reg.Start(ctx); err != nil {
			logx.Errorf("etcd register failed: %v", err)
		} else {
			logx.Infof("gateway registered to etcd key=%q addr=%s", base.Etcd.Key, addr)
		}
	} else if err != nil {
		logx.Errorf("etcd reg init failed: %v", err)
	}

	rds := redis.MustNewRedis(base.Redis)

	// 创建 gateway 服务
	opts := []gateway.Option{
		gateway.WithMiddleware(middleware.Healthz),
	}
	rl := base.RateLimit.Normalize()
	switch rl.Mode {
	case "off":
		// no limiter
	case "memory":
		opts = append(opts, gateway.WithMiddleware(middleware.RateLimitWithConf(rl)))
	default: // "redis"
		opts = append(opts, gateway.WithMiddleware(middleware.RedisRateLimitWithConf(rds, rl)))
	}

	gw := gateway.MustNewServer(gwConf, opts...)
	defer gw.Stop()
	go gw.Start()

	// 监听信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	gw.Stop()
	if reg != nil {
		reg.Stop(context.Background())
	}
}
