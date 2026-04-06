package snowflake

import (
	"errors"
	"sync"
	"time"
)

// Generator 雪花算法生成器
type Generator struct {
	mu sync.Mutex // 互斥锁

	epochMs int64  // 41 bits 时间戳 开始时间戳
	node    uint16 // 10 bits 节点
	lastMs  int64  // 41 bits 时间戳 上次时间戳
	seq     uint16 // 12 bits 序列号
}

const (
	nodeBits = 10 // 10 bits 节点
	seqBits  = 12 // 12 bits 序列

	maxNode = (1 << nodeBits) - 1 // 1023 最大节点数
	maxSeq  = (1 << seqBits) - 1  // 4095 最大序列数
)

// New 创建雪花算法生成器
func New(node int, epoch time.Time) (*Generator, error) {
	// 如果节点小于 0 或大于最大节点数，则返回错误
	if node < 0 || node > maxNode {
		return nil, errors.New("snowflake: node must be in [0,1023]")
	}
	epochMs := epoch.UnixMilli()
	if epochMs <= 0 {
		return nil, errors.New("snowflake: invalid epoch")
	}
	return &Generator{
		epochMs: epochMs,
		node:    uint16(node),
		lastMs:  -1,
		seq:     0,
	}, nil
}

// Next 生成下一个唯一ID
func (g *Generator) Next() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	for {
		nowMs := time.Now().UnixMilli()
		if nowMs < g.lastMs {
			// clock moved backwards; wait until it catches up
			time.Sleep(time.Duration(g.lastMs-nowMs) * time.Millisecond)
			continue
		}

		if nowMs == g.lastMs {
			if g.seq < maxSeq {
				g.seq++
				return g.pack(nowMs, g.seq)
			}
			// sequence overflow in same ms: wait next ms
			for nowMs <= g.lastMs {
				time.Sleep(200 * time.Microsecond)
				nowMs = time.Now().UnixMilli()
			}
			// new ms
			g.lastMs = nowMs
			g.seq = 0
			return g.pack(nowMs, g.seq)
		}

		// new ms
		g.lastMs = nowMs
		g.seq = 0
		return g.pack(nowMs, g.seq)
	}
}

// pack 打包时间戳和序列号
func (g *Generator) pack(nowMs int64, seq uint16) uint64 {
	ts := uint64(nowMs - g.epochMs)
	return (ts << (nodeBits + seqBits)) | (uint64(g.node) << seqBits) | uint64(seq)
}
