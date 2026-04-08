package wal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bld-backend/core/model"
)

func TestCodecRoundTripAndCRC(t *testing.T) {
	rec := Record{
		LSN:     1,
		Type:    RecordAddOrder,
		TsMs:    time.Now().UnixMilli(),
		Payload: []byte(`{"ok":true}`),
	}
	raw := encodeFrame(rec)
	// Corrupt one byte in payload area and ensure decode fails.
	raw[len(raw)-1] ^= 0x1
	f, err := os.CreateTemp("", "wal-corrupt-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeFrameFrom(f); err == nil {
		t.Fatal("expected crc decode error")
	}
}

func TestWriterAppendBatchAndReadLastLSN(t *testing.T) {
	dir := t.TempDir()
	walPath := filepath.Join(dir, "exchange.wal")
	w, err := New(walPath, 5*time.Millisecond, 16)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	w.Start(context.Background())

	msg := model.SpotOrderKafkaMsg{OrderID: 99, MarketID: 1, Status: "PENDING", RemainingQty: "3"}
	raw, _ := json.Marshal(msg)
	if _, err := w.AppendBatch(w.NextTxID(), []BatchEntry{{Type: RecordAddOrder, TsMs: time.Now().UnixMilli(), Payload: raw}}); err != nil {
		t.Fatal(err)
	}
	lastLSN, err := readLastLSN(walPath)
	if err != nil {
		t.Fatal(err)
	}
	if lastLSN == 0 {
		t.Fatalf("expected lastLSN > 0, got %d", lastLSN)
	}
}
