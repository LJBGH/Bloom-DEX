package wal

import "errors"

const (
	frameMagic   uint32 = 0x57414c32 // "WAL2"
	frameVersion uint8  = 1
)

var (
	ErrLegacyJSONWAL = errors.New("legacy JSON wal format is not supported, clear wal/ckpt and restart")
	ErrCorruptedWAL  = errors.New("corrupted wal frame")
)

type RecordType uint8

const (
	RecordAddOrder    RecordType = 1 // 添加订单
	RecordTrade       RecordType = 2 // 交易
	RecordUpdateOrder RecordType = 3 // 更新订单
	RecordRemoveOrder RecordType = 4 // 删除订单
	RecordCancelOrder RecordType = 5 // 取消订单
	RecordFilledOrder RecordType = 6 // 成交订单
)

type Record struct {
	LSN     uint64     // 日志序号
	Type    RecordType // 类型
	TsMs    int64      // 时间戳
	Payload []byte     // 负载
}

type BatchEntry struct {
	Type    RecordType
	TsMs    int64
	Payload []byte
}

type LegacyEvent struct {
	Type     string `json:"type"`
	OrderID  uint64 `json:"order_id,omitempty"`
	MarketID int    `json:"market_id,omitempty"`
	TradeID  uint64 `json:"trade_id,omitempty"`
	TsMs     int64  `json:"ts_ms"`
	Payload  any    `json:"payload,omitempty"`
}
