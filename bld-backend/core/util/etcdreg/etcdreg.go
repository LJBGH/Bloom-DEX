package etcdreg

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type Config struct {
	Hosts []string `json:"Hosts,optional"`
	Key   string   `json:"Key,optional"`
	TTL   int64    `json:"TTL,default=10"`
}

type Registrar struct {
	cli   *clientv3.Client
	kv    clientv3.KV
	ttl   int64
	key   string
	val   string
	lease clientv3.Lease
	id    clientv3.LeaseID
	stop  context.CancelFunc
}

// 创建注册服务
func New(cfg Config, val string) (*Registrar, error) {
	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("etcd hosts is empty")
	}
	if cfg.Key == "" {
		return nil, fmt.Errorf("etcd key is empty")
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 10
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Hosts,
		DialTimeout: 3 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return &Registrar{
		cli:   cli,
		kv:    clientv3.NewKV(cli),
		lease: clientv3.NewLease(cli),
		ttl:   ttl,
		key:   cfg.Key,
		val:   val,
	}, nil
}

// 开始注册服务
func (r *Registrar) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	r.stop = cancel

	grant, err := r.lease.Grant(ctx, r.ttl)
	if err != nil {
		return err
	}
	r.id = grant.ID

	if _, err := r.kv.Put(ctx, r.key, r.val, clientv3.WithLease(r.id)); err != nil {
		return err
	}

	ch, err := r.lease.KeepAlive(ctx, r.id)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
			}
		}
	}()

	return nil
}

// 停止注册服务
func (r *Registrar) Stop(ctx context.Context) {
	if r.stop != nil {
		r.stop()
	}
	if r.id != 0 {
		_, _ = r.lease.Revoke(ctx, r.id)
	}
	if r.cli != nil {
		_ = r.cli.Close()
	}
}
