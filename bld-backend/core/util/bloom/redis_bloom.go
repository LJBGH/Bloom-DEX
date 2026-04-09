package bloom

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisBloom 存储 Bloom 位集在 Redis 中使用 SETBIT/GETBIT。
type RedisBloom struct {
	rdb   *redis.Client
	key   string
	mBits uint64
	k     uint8
}

// 创建 Redis 支持的 Bloom 过滤器
func NewRedisBloom(rdb *redis.Client, key string, mBits uint64, k uint8) (*RedisBloom, error) {
	// 检查 Redis 客户端是否为空
	if rdb == nil {
		return nil, errors.New("bloom: redis client is nil")
	}
	// 检查 Redis 键是否为空
	if key == "" {
		return nil, errors.New("bloom: redis key is empty")
	}
	// 检查 mBits 是否为 0
	if mBits == 0 {
		return nil, errors.New("bloom: mBits must be > 0")
	}
	// 检查 k 是否为 0
	if k == 0 {
		return nil, errors.New("bloom: k must be > 0")
	}
	// 创建 RedisBloom 结构体
	return &RedisBloom{
		rdb:   rdb,
		key:   key,
		mBits: mBits,
		k:     k,
	}, nil
}

// 根据预期插入数和目标误判率创建 Redis 支持的 Bloom 过滤器
func NewRedisBloomWithEstimates(rdb *redis.Client, key string, n uint64, p float64) (*RedisBloom, error) {
	// 计算 Bloom 参数 (mBits, k) 用于 n 预期插入数和误判率 p
	mBits, k, err := Params(n, p)
	if err != nil {
		return nil, err
	}
	return NewRedisBloom(rdb, key, mBits, k)
}

// 添加字符串键
func (b *RedisBloom) AddString(ctx context.Context, s string) error {
	return b.AddBytes(ctx, []byte(s))
}

// 检查字符串键是否可能存在
func (b *RedisBloom) ExistsString(ctx context.Context, s string) (bool, error) {
	return b.ExistsBytes(ctx, []byte(s))
}

// 添加字节切片键
func (b *RedisBloom) AddBytes(ctx context.Context, data []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	h1, h2 := hash2(data)

	pipe := b.rdb.Pipeline()
	for i := uint8(0); i < b.k; i++ {
		pos := (h1 + uint64(i)*h2) % b.mBits
		pipe.SetBit(ctx, b.key, int64(pos), 1)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// 检查字节切片键是否可能存在
func (b *RedisBloom) ExistsBytes(ctx context.Context, data []byte) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h1, h2 := hash2(data)

	pipe := b.rdb.Pipeline()
	cmds := make([]*redis.IntCmd, 0, int(b.k))
	for i := uint8(0); i < b.k; i++ {
		pos := (h1 + uint64(i)*h2) % b.mBits
		cmds = append(cmds, pipe.GetBit(ctx, b.key, int64(pos)))
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	for _, c := range cmds {
		v, err := c.Result()
		if err != nil {
			return false, err
		}
		if v == 0 {
			return false, nil
		}
	}
	return true, nil
}

// 调试信息
func (b *RedisBloom) DebugInfo() string {
	return fmt.Sprintf("RedisBloom{key=%q mBits=%d k=%d}", b.key, b.mBits, b.k)
}

// 哈希函数
func hash2(data []byte) (uint64, uint64) {
	sum := sha256.Sum256(data)
	h1 := binary.LittleEndian.Uint64(sum[0:8])
	h2 := binary.LittleEndian.Uint64(sum[8:16])
	//
	if h2 == 0 {
		h2 = 0x9e3779b97f4a7c15
	}
	return h1, h2
}
