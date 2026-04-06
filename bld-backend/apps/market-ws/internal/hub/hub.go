package hub

import (
	"sync"

	"github.com/zeromicro/go-zero/core/logx"
)

// Client 由 ws 包构造并注册到 Hub；Send 为带缓冲发送队列。
type Client struct {
	Send    chan []byte
	Markets map[int]struct{}
}

// Hub 按 market_id 维护订阅者，并缓存最近一次深度 JSON（供 HTTP 快照）。
type Hub struct {
	mu   sync.RWMutex
	subs map[int]map[*Client]struct{}
	last map[int][]byte
}

// New 创建一个新的 Hub
func New() *Hub {
	return &Hub{
		subs: make(map[int]map[*Client]struct{}),
		last: make(map[int][]byte),
	}
}

// SetLastDepth 设置市场最后一次深度
func (h *Hub) SetLastDepth(marketID int, rawJSON []byte) {
	h.mu.Lock()
	h.last[marketID] = rawJSON
	h.mu.Unlock()
}

// LastDepth 获取市场最后一次深度
func (h *Hub) LastDepth(marketID int) ([]byte, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	b, ok := h.last[marketID]
	return b, ok
}

// Subscribe 订阅市场
func (h *Hub) Subscribe(marketID int, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[marketID] == nil {
		h.subs[marketID] = make(map[*Client]struct{})
	}
	h.subs[marketID][c] = struct{}{}
	if c.Markets == nil {
		c.Markets = make(map[int]struct{})
	}
	c.Markets[marketID] = struct{}{}
}

// Unsubscribe 取消订阅
func (h *Hub) Unsubscribe(marketID int, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.subs[marketID]; ok {
		delete(m, c)
		if len(m) == 0 {
			delete(h.subs, marketID)
		}
	}
	if c.Markets != nil {
		delete(c.Markets, marketID)
	}
}

// UnsubscribeAll 取消所有订阅
func (h *Hub) UnsubscribeAll(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for mid := range c.Markets {
		if m, ok := h.subs[mid]; ok {
			delete(m, c)
			if len(m) == 0 {
				delete(h.subs, mid)
			}
		}
	}
	c.Markets = make(map[int]struct{})
}

// Broadcast 广播消息到所有订阅者
func (h *Hub) Broadcast(marketID int, payload []byte) {
	h.mu.RLock()
	var list []*Client
	if m, ok := h.subs[marketID]; ok {
		for c := range m {
			list = append(list, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range list {
		select {
		case c.Send <- payload:
		default:
			logx.Errorf("market-ws: client send buffer full, drop message market_id=%d (slow consumer)", marketID)
		}
	}
}
