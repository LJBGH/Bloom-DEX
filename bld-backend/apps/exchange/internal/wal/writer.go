package wal

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type Event = LegacyEvent

// Writer 订单簿持久化写入器。
type Writer struct {
	mu      sync.Mutex
	file    *os.File       // 文件
	writer  *bufio.Writer  // 写入器
	wg      sync.WaitGroup // 等待组
	flushIn time.Duration  // 刷新间隔
	lsn     uint64         // 当前日志序号
	txID    uint64
	ctx     context.Context
	cancel  context.CancelFunc
}

// New 创建订单簿持久化写入器。
// - path：文件路径
// - flushInterval：刷新间隔
// - queueSize：队列大小
// 返回：
// - *Writer：订单簿持久化写入器
// - error：错误
func New(path string, flushInterval time.Duration, queueSize int) (*Writer, error) {
	// 如果刷新间隔小于等于 0，则设置为 10 毫秒
	if flushInterval <= 0 {
		flushInterval = 10 * time.Millisecond
	}
	// 如果队列大小小于等于 0，则设置为 4096
	if queueSize <= 0 {
		queueSize = 4096
	}
	// 创建文件目录
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	legacy, err := isLegacyJSONWAL(f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if legacy {
		_ = f.Close()
		return nil, ErrLegacyJSONWAL
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, err
	}
	lastLSN, err := readLastLSN(path)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		_ = f.Close()
		return nil, err
	}
	// 创建订单簿持久化写入器
	cctx, cancel := context.WithCancel(context.Background())
	w := &Writer{
		file:    f,
		writer:  bufio.NewWriterSize(f, 1<<20),
		flushIn: flushInterval,
		lsn:     lastLSN,
		txID:    0,
		ctx:     cctx,
		cancel:  cancel,
	}
	return w, nil
}

// Start 启动订单簿持久化写入器。
func (w *Writer) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		tk := time.NewTicker(w.flushIn)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				_ = w.Sync()
			case <-ctx.Done():
				_ = w.Sync()
				return
			case <-w.ctx.Done():
				_ = w.Sync()
				return
			}
		}
	}()
}

func (w *Writer) NextTxID() uint64 {
	return atomic.AddUint64(&w.txID, 1)
}

// AppendBatch 原子追加一个事务批次并刷盘。
func (w *Writer) AppendBatch(txID uint64, entries []BatchEntry) (uint64, error) {
	if len(entries) == 0 {
		return atomic.LoadUint64(&w.lsn), nil
	}
	if txID == 0 {
		txID = w.NextTxID()
	}
	_ = txID

	w.mu.Lock()
	defer w.mu.Unlock()
	var last uint64
	for _, e := range entries {
		rec := Record{
			LSN:     atomic.AddUint64(&w.lsn, 1),
			Type:    e.Type,
			TsMs:    e.TsMs,
			Payload: e.Payload,
		}
		frame := encodeFrame(rec)
		if _, err := w.writer.Write(frame); err != nil {
			return last, err
		}
		last = rec.LSN
	}
	if err := w.writer.Flush(); err != nil {
		return last, err
	}
	if err := w.file.Sync(); err != nil {
		return last, err
	}
	return last, nil
}

// Append 兼容旧调用：将旧事件按交易事件直接写入（不再写 Legacy/事务包裹）。
func (w *Writer) Append(ev Event) {
	raw, err := json.Marshal(ev)
	if err != nil {
		return
	}
	_, _ = w.AppendBatch(w.NextTxID(), []BatchEntry{{Type: RecordTrade, TsMs: time.Now().UnixMilli(), Payload: raw}})
}

// readLastLSN 读取最后一个 LSN。
func readLastLSN(path string) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer func() { _ = f.Close() }()

	legacy, err := isLegacyJSONWAL(f)
	if err != nil {
		return 0, err
	}
	if legacy {
		return 0, ErrLegacyJSONWAL
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	var last uint64
	for {
		rec, err := decodeFrameFrom(f)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			if err == ErrCorruptedWAL {
				break
			}
			return 0, err
		}
		if rec.LSN > last {
			last = rec.LSN
		}
	}
	return last, nil
}

// Sync 强制将当前缓冲区刷盘（包含此前已入队的事件）。
func (w *Writer) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.writer.Flush(); err != nil {
		return err
	}
	return w.file.Sync()
}

// Close 关闭订单簿持久化写入器。
func (w *Writer) Close() error {
	w.cancel()
	w.wg.Wait()
	_ = w.Sync()
	return w.file.Close()
}

// isLegacyJSONWAL 检查是否为旧的 JSON WAL。
func isLegacyJSONWAL(f *os.File) (bool, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	buf := make([]byte, 1)
	for {
		n, err := f.Read(buf)
		if err != nil {
			if err == io.EOF {
				return false, nil
			}
			return false, err
		}
		if n == 0 {
			return false, nil
		}
		b := buf[0]
		if b == '{' {
			return true, nil
		}
		if b == '\n' || b == '\r' || b == '\t' || b == ' ' {
			continue
		}
		return false, nil
	}
}
