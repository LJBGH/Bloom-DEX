package bloom

import (
	"errors"
	"hash/maphash"
	"math"
	"sync"
)

// Bloom 过滤器
type Filter struct {
	mBits uint64 // number of bits
	k     uint8  // number of hash rounds

	words []uint64

	mu sync.RWMutex

	seed1 maphash.Seed
	seed2 maphash.Seed
}

// 创建 Bloom 过滤器
func New(mBits uint64, k uint8) (*Filter, error) {
	if mBits == 0 {
		return nil, errors.New("bloom: mBits must be > 0")
	}
	if k == 0 {
		return nil, errors.New("bloom: k must be > 0")
	}
	words := make([]uint64, (mBits+63)/64)
	return &Filter{
		mBits: mBits,
		k:     k,
		words: words,
		seed1: maphash.MakeSeed(),
		seed2: maphash.MakeSeed(),
	}, nil
}

// 根据预期插入数和目标误判率创建 Bloom 过滤器
func NewWithEstimates(n uint64, p float64) (*Filter, error) {
	mBits, k, err := Params(n, p)
	if err != nil {
		return nil, err
	}
	return New(mBits, k)
}

// 计算 Bloom 参数 (mBits, k) 用于 n 预期插入数和误判率 p
func Params(n uint64, p float64) (mBits uint64, k uint8, err error) {
	if n == 0 {
		return 0, 0, errors.New("bloom: n must be > 0")
	}
	if !(p > 0 && p < 1) {
		return 0, 0, errors.New("bloom: p must be in (0,1)")
	}

	// m = -(n * ln p) / (ln 2)^2 计算所需位数
	ln2 := math.Ln2
	m := -float64(n) * math.Log(p) / (ln2 * ln2)
	if m < 1 {
		m = 1
	}
	// k = (m/n) * ln 2 计算哈希函数数量
	kk := (m / float64(n)) * ln2
	if kk < 1 {
		kk = 1
	}

	// 向上取整位数，限制 k 为合理最大值
	mBits = uint64(math.Ceil(m))
	if mBits == 0 {
		mBits = 1
	}
	kInt := int(math.Round(kk))
	if kInt < 1 {
		kInt = 1
	}
	if kInt > 64 {
		kInt = 64
	}
	return mBits, uint8(kInt), nil
}

// 添加字符串键
func (f *Filter) AddString(s string) {
	f.AddBytes([]byte(s))
}

// 检查字符串键是否可能存在
func (f *Filter) ExistsString(s string) bool {
	return f.ExistsBytes([]byte(s))
}

// 添加字节切片键
func (f *Filter) AddBytes(b []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()

	h1, h2 := f.hash(b)
	for i := uint8(0); i < f.k; i++ {
		pos := f.pos(h1, h2, i)
		f.setBit(pos)
	}
}

// 检查字节切片键是否可能存在
func (f *Filter) ExistsBytes(b []byte) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	h1, h2 := f.hash(b)
	for i := uint8(0); i < f.k; i++ {
		pos := f.pos(h1, h2, i)
		if !f.getBit(pos) {
			return false
		}
	}
	return true
}

// 哈希函数
func (f *Filter) hash(b []byte) (uint64, uint64) {
	var h maphash.Hash

	h.SetSeed(f.seed1)
	_, _ = h.Write(b)
	h1 := h.Sum64()

	h.Reset()
	h.SetSeed(f.seed2)
	_, _ = h.Write(b)
	h2 := h.Sum64()

	// Ensure h2 is non-zero to make (h1 + i*h2) advance.
	if h2 == 0 {
		h2 = 0x9e3779b97f4a7c15
	}
	return h1, h2
}

// 计算位置
func (f *Filter) pos(h1, h2 uint64, i uint8) uint64 {
	// double hashing: (h1 + i*h2) mod m
	// use uint64 math; modulo by mBits
	return (h1 + uint64(i)*h2) % f.mBits
}

// 设置位
func (f *Filter) setBit(bit uint64) {
	word := bit / 64
	off := bit % 64
	f.words[word] |= (uint64(1) << off)
}

// 获取位
func (f *Filter) getBit(bit uint64) bool {
	word := bit / 64
	off := bit % 64
	return (f.words[word] & (uint64(1) << off)) != 0
}
