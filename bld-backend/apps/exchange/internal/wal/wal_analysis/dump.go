package wal_analysis

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strings"

	"bld-backend/apps/exchange/internal/wal"
)

type DumpRow struct {
	LSN        uint64 `json:"lsn"`
	Type       uint8  `json:"type"`
	TypeName   string `json:"type_name"`
	TsMs       int64  `json:"ts_ms"`
	PayloadLen int    `json:"payload_len"`
	Payload    any    `json:"payload,omitempty"`
	PayloadB64 string `json:"payload_b64,omitempty"`
}

func RecordTypeName(t wal.RecordType) string {
	switch t {
	case wal.RecordAddOrder:
		return "ADD_ORDER"
	case wal.RecordTrade:
		return "TRADE"
	case wal.RecordUpdateOrder:
		return "UPDATE_ORDER"
	case wal.RecordRemoveOrder:
		return "REMOVE_ORDER"
	case wal.RecordCancelOrder:
		return "CANCEL_ORDER"
	default:
		return "UNKNOWN"
	}
}

// DumpRows 扫描二进制 WAL 并返回可读的行。
func DumpRows(path string, fromLSN uint64, limit int) ([]DumpRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	legacy, err := isLegacyJSONWAL(f)
	if err != nil {
		return nil, err
	}
	if legacy {
		return nil, wal.ErrLegacyJSONWAL
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	rows := make([]DumpRow, 0, 256)
	for {
		rec, err := decodeFrameFrom(f)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF || err == wal.ErrCorruptedWAL {
				break
			}
			return nil, err
		}
		if rec.LSN <= fromLSN {
			continue
		}
		row := DumpRow{
			LSN:        rec.LSN,
			Type:       uint8(rec.Type),
			TypeName:   RecordTypeName(rec.Type),
			TsMs:       rec.TsMs,
			PayloadLen: len(rec.Payload),
		}
		if len(rec.Payload) > 0 {
			var obj any
			dec := json.NewDecoder(bytes.NewReader(rec.Payload))
			dec.UseNumber()
			if err := dec.Decode(&obj); err == nil {
				// Keep payload object readable while forcing order_id first.
				row.Payload = payloadWithOrderIDFirst(rec.Payload, obj)
			} else {
				row.PayloadB64 = base64.StdEncoding.EncodeToString(rec.Payload)
			}
		}
		rows = append(rows, row)
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows, nil
}

// 将 payload 中的 order_id 移到最前面。
func payloadWithOrderIDFirst(raw []byte, fallback any) any {
	trimmed := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(trimmed, "{") {
		return fallback
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fallback
	}
	orderID, ok := obj["order_id"]
	if !ok {
		return fallback
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		if k == "order_id" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b bytes.Buffer
	b.WriteByte('{')
	b.WriteString(`"order_id":`)
	b.Write(orderID)
	for _, k := range keys {
		b.WriteByte(',')
		keyJSON, _ := json.Marshal(k)
		b.Write(keyJSON)
		b.WriteByte(':')
		b.Write(obj[k])
	}
	b.WriteByte('}')
	return json.RawMessage(b.Bytes())
}
