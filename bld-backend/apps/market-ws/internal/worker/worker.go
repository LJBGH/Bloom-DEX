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
	"bld-backend/apps/market-ws/internal/kline"
	wsh "bld-backend/apps/market-ws/internal/ws"
	"bld-backend/core/enum"
	"bld-backend/core/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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

	conn := sqlx.NewMysql(w.cfg.Mysql.DataSource)
	klineStore := kline.NewStore(conn)
	// 若未预聚合过，则用 spot_trades 做一次最小启动补全，避免仅依赖 Kafka offset。
	if cnt, err := klineStore.Count(ctx); err == nil && cnt == 0 {
		logx.Infof("spot_klines empty (cnt=0), bootstrapping from spot_trades")
		if err := klineStore.BootstrapFromSpotTrades(ctx, 5000); err != nil {
			logx.Errorf("spot_klines bootstrap failed: %v", err)
		}
	}

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
			if err := consumer.StartTradeConsumer(ctx, w.cfg.Kafka.Brokers, tradeGroupID, tradeTopic, w.hub, klineStore); err != nil && ctx.Err() == nil {
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
	depthHandler := func(rw http.ResponseWriter, r *http.Request) {
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
	}
	mux.HandleFunc("/api/v1/depth", depthHandler)
	mux.HandleFunc("/api/marketws/api/v1/depth", depthHandler)

	// 处理 K线历史请求
	klinesHandler := func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		mid, err := strconv.Atoi(q.Get("market_id"))
		if err != nil || mid <= 0 {
			http.Error(rw, "invalid market_id", http.StatusBadRequest)
			return
		}
		interval := q.Get("interval")
		if interval == "" {
			interval = "1m"
		}
		fromMs, _ := strconv.ParseInt(q.Get("from_ms"), 10, 64)
		toMs, _ := strconv.ParseInt(q.Get("to_ms"), 10, 64)
		limit, _ := strconv.Atoi(q.Get("limit"))
		rows, err := klineStore.List(r.Context(), mid, interval, fromMs, toMs, limit)
		if err != nil {
			logx.Errorf("market-ws klines list error: %v", err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		raw, err := json.Marshal(struct {
			Items []kline.Row `json:"items"`
		}{Items: rows})
		if err != nil {
			logx.Errorf("market-ws klines json marshal error: %v", err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		_, _ = rw.Write(raw)
	}
	mux.HandleFunc("/api/v1/klines", klinesHandler)
	mux.HandleFunc("/api/marketws/api/v1/klines", klinesHandler)

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
