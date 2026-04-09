package bloom

import (
	"strconv"
	"testing"
)

func TestFilter_AddAndExists(t *testing.T) {
	f, err := NewWithEstimates(10_000, 0.01)
	if err != nil {
		t.Fatalf("NewWithEstimates: %v", err)
	}

	// 插入前：肯定不存在
	if f.ExistsString("hello") {
		t.Fatalf("expected hello to be definitely absent before insert")
	}

	// 插入后：肯定存在
	f.AddString("hello")
	if !f.ExistsString("hello") {
		t.Fatalf("expected hello to be present after insert")
	}

	// 插入一堆
	for i := 0; i < 5000; i++ {
		f.AddString("k:" + strconv.Itoa(i))
	}
	for i := 0; i < 5000; i++ {
		if !f.ExistsString("k:" + strconv.Itoa(i)) {
			t.Fatalf("expected inserted key to exist: %d", i)
		}
	}
}

func TestParams(t *testing.T) {
	mb, k, err := Params(1000, 0.01)
	if err != nil {
		t.Fatalf("Params: %v", err)
	}
	if mb == 0 || k == 0 {
		t.Fatalf("unexpected params: mBits=%d k=%d", mb, k)
	}
}
