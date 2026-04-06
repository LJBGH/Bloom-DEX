package ws

import (
	"encoding/json"
	"net/http"
	"time"

	"bld-backend/apps/market-ws/internal/hub"
	"bld-backend/apps/market-ws/internal/wire"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin: func(*http.Request) bool {
		return true
	},
}

type ctrlMsg struct {
	Op       string `json:"op"`
	MarketID int    `json:"market_id"`
}

// Serve 将连接升级为 WebSocket，处理 subscribe / unsubscribe。
func Serve(h *hub.Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logx.Errorf("ws upgrade: %v", err)
		return
	}

	c := &hub.Client{
		Send:    make(chan []byte, 2048),
		Markets: make(map[int]struct{}),
	}

	go writePump(conn, c.Send)
	readPump(h, conn, c)
}

// readPump 读取 WebSocket 消息，处理 subscribe / unsubscribe。
func readPump(h *hub.Hub, conn *websocket.Conn, c *hub.Client) {
	defer func() {
		h.UnsubscribeAll(c)
		_ = conn.Close()
	}()
	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logx.Errorf("ws read: %v", err)
			}
			break
		}
		var m ctrlMsg
		if err := json.Unmarshal(data, &m); err != nil || m.MarketID <= 0 {
			continue
		}
		switch m.Op {
		case "subscribe":
			h.Subscribe(m.MarketID, c)
			if raw, ok := h.LastDepth(m.MarketID); ok {
				if env, err := wire.DepthEvent(raw); err == nil {
					select {
					case c.Send <- env:
					default:
					}
				}
			}
		case "unsubscribe":
			h.Unsubscribe(m.MarketID, c)
		}
	}
}

// writePump 写入 WebSocket 消息，发送心跳和消息。
func writePump(conn *websocket.Conn, send <-chan []byte) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = conn.Close()
	}()
	for {
		select {
		case msg, ok := <-send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(msg); err != nil {
				_ = w.Close()
				return
			}
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
