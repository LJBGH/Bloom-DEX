package wire

import "encoding/json"

// DepthEvent 与 Kafka 内层 JSON 一致的外层推送格式（WebSocket 文本帧）。
func DepthEvent(inner []byte) ([]byte, error) {
	return json.Marshal(struct {
		Channel string          `json:"channel"`
		Data    json.RawMessage `json:"data"`
	}{Channel: "depth", Data: inner})
}

// TradeEvent 公开成交外层格式。
func TradeEvent(inner []byte) ([]byte, error) {
	return json.Marshal(struct {
		Channel string          `json:"channel"`
		Data    json.RawMessage `json:"data"`
	}{Channel: "trade", Data: inner})
}
