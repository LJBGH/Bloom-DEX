package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"bld-backend/apps/market-ws/internal/config"
	"bld-backend/apps/market-ws/internal/consumer"
	"bld-backend/apps/market-ws/internal/hub"
	wsh "bld-backend/apps/market-ws/internal/ws"
	"bld-backend/core/enum"
	"bld-backend/core/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type Worker struct {
	cfg    config.Config
	hub    *hub.Hub
	srv    *http.Server
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(cfg config.Config) *Worker {
	return &Worker{cfg: cfg, hub: hub.New()}
}

// Start 启动 worker
func (w *Worker) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel

	// 获取深度主题
	depthTopic := w.cfg.Kafka.DepthTopic
	if depthTopic == "" {
		depthTopic = enum.TopicDepthDelta
	}
	tradeTopic := w.cfg.Kafka.TradeTopic
	if tradeTopic == "" {
		tradeTopic = enum.TopicMarketTrade
	}
	// 获取 groupID
	groupID := w.cfg.Kafka.GroupID
	if groupID == "" {
		groupID = "market-ws-depth"
	}
	tradeGroupID := w.cfg.Kafka.TradeGroupID
	if tradeGroupID == "" {
		tradeGroupID = "market-ws-trade"
	}

	// 启动 Kafka 消费
	if len(w.cfg.Kafka.Brokers) > 0 {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			logx.Infof("market-ws consuming topic=%q group=%q", depthTopic, groupID)
			if err := consumer.StartDepthConsumer(ctx, w.cfg.Kafka.Brokers, groupID, depthTopic, w.hub); err != nil && ctx.Err() == nil {
				logx.Errorf("market-ws kafka consumer stopped: %v", err)
			}
		}()
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			logx.Infof("market-ws consuming topic=%q group=%q", tradeTopic, tradeGroupID)
			if err := consumer.StartTradeConsumer(ctx, w.cfg.Kafka.Brokers, tradeGroupID, tradeTopic, w.hub); err != nil && ctx.Err() == nil {
				logx.Errorf("market-ws trade kafka consumer stopped: %v", err)
			}
		}()
	} else {
		logx.Error("market-ws: Kafka.Brokers empty, depth push disabled")
	}

	// 创建 HTTP 路由
	mux := http.NewServeMux()
	// 处理 WebSocket 请求
	mux.HandleFunc("/ws", func(rw http.ResponseWriter, r *http.Request) {
		wsh.Serve(w.hub, rw, r)
	})
	// 处理深度请求
	mux.HandleFunc("/api/v1/depth", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query().Get("market_id")
		mid, err := strconv.Atoi(q)
		if err != nil || mid <= 0 {
			http.Error(rw, "invalid market_id", http.StatusBadRequest)
			return
		}
		raw, ok := w.hub.LastDepth(mid)
		if !ok {
			// 尚无 Kafka 深度时仍返回 200 + 空盘口，避免前端/直连误以为路由 404
			empty := model.MarketDepthKafkaMsg{
				MarketID: mid,
				Seq:      0,
				TsMs:     time.Now().UnixMilli(),
				Bids:     make([]model.DepthPriceLevel, 0),
				Asks:     make([]model.DepthPriceLevel, 0),
			}
			raw, err := json.Marshal(empty)
			if err != nil {
				http.Error(rw, "internal error", http.StatusInternalServerError)
				return
			}
			rw.Header().Set("Content-Type", "application/json")
			_, _ = rw.Write(raw)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		_, _ = rw.Write(raw)
	})

	// 启动 HTTP 服务器
	addr := w.cfg.ListenOn
	// 如果地址为空，则使用默认地址
	if addr == "" {
		addr = "0.0.0.0:9201"
	}
	w.srv = &http.Server{Addr: addr, Handler: mux}
	w.wg.Add(1)
	// 启动 HTTP 服务器
	go func() {
		defer w.wg.Done()
		logx.Infof("market-ws http listen %s", addr)
		if err := w.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logx.Errorf("market-ws http: %v", err)
		}
	}()

	logx.Infof("worker %s bootstrap completed", w.cfg.Name)
}

// Stop 停止 worker
func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	if w.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = w.srv.Shutdown(ctx)
		cancel()
	}
	w.wg.Wait()
	logx.Infof("worker %s shutdown completed", w.cfg.Name)
}
