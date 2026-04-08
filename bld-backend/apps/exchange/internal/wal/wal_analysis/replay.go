package wal_analysis

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"bld-backend/apps/exchange/internal/wal"
	"bld-backend/core/model"
)

// OrderEvent WAL 中可回放的 order 事件。
type OrderEvent struct {
	LSN uint64
	Msg *model.SpotOrderKafkaMsg
}

type checkpoint struct {
	LastLSN uint64 `json:"last_lsn"`
}

// 读取 checkpoint。
func ReadCheckpoint(path string) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	defer func() { _ = f.Close() }()
	var cp checkpoint
	if err := json.NewDecoder(f).Decode(&cp); err != nil {
		return 0, err
	}
	return cp.LastLSN, nil
}

// 持久化 checkpoint。
func WriteCheckpoint(path string, lastLSN uint64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return json.NewEncoder(f).Encode(checkpoint{LastLSN: lastLSN})
}

// LoadOrderEventsSince 读取 WAL 中 LSN 大于 fromLSN 的 order 事件（按文件顺序）。
// 同时返回文件中的最大 LSN（用于推进 checkpoint）。
func LoadOrderEventsSince(path string, fromLSN uint64) ([]OrderEvent, uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fromLSN, nil
		}
		return nil, fromLSN, err
	}
	defer func() { _ = f.Close() }()

	legacy, err := isLegacyJSONWAL(f)
	if err != nil {
		return nil, fromLSN, err
	}
	if legacy {
		return nil, fromLSN, wal.ErrLegacyJSONWAL
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fromLSN, err
	}

	events := make([]OrderEvent, 0, 1024)
	maxLSN := fromLSN

	for {
		rec, err := decodeFrameFrom(f)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			if err == wal.ErrCorruptedWAL {
				break
			}
			return nil, maxLSN, err
		}
		if rec.LSN > maxLSN {
			maxLSN = rec.LSN
		}
		if rec.LSN <= fromLSN {
			continue
		}
		switch rec.Type {
		case wal.RecordAddOrder:
			var msg model.SpotOrderKafkaMsg
			if err := json.Unmarshal(rec.Payload, &msg); err == nil && msg.OrderID > 0 {
				events = append(events, OrderEvent{LSN: rec.LSN, Msg: &msg})
			}
		}
	}
	return events, maxLSN, nil
}
