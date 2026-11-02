//go:build windows

package windows

import (
	"image"
	"sync"
	"testing"
	"time"
)

// TestBufferPoolReuse 验证缓冲会被复用：稳态下不再分配。
func TestBufferPoolReuse(t *testing.T) {
	pool := NewBufferPool(1920, 1080)
	for i := 0; i < 5; i++ {
		b := pool.Acquire()
		b.Release()
	}
	st := pool.Stats()
	if st.Allocs != 1 {
		t.Fatalf("5 次取还共分配 %d 块，应当只分配 1 块", st.Allocs)
	}
	if st.Free != 1 || st.Live != 0 {
		t.Fatalf("free=%d live=%d，应当 free=1 live=0", st.Free, st.Live)
	}
}

// TestBufferPoolConcurrentRetain 验证引用计数：多个消费者可以同时持有同一帧。
func TestBufferPoolConcurrentRetain(t *testing.T) {
	pool := NewBufferPool(1920, 1080)
	b := pool.Acquire()

	const workers = 3
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[*image.RGBA]int{}
	for i := 0; i < workers; i++ {
		b.Retain()
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			seen[b.Image()]++
			mu.Unlock()
			b.Release()
		}()
	}
	b.Release() // 主流程这一份
	wg.Wait()

	if len(seen) != 1 {
		t.Fatalf("worker 看到的图像有 %d 个不同指针，应当共用同一块缓冲", len(seen))
	}
	st := pool.Stats()
	if st.Live != 0 {
		t.Fatalf("全部释放后 live=%d，应当归零", st.Live)
	}
	if st.Free != 1 {
		t.Fatalf("全部释放后 free=%d，应当回到池里", st.Free)
	}
}

// TestBufferPoolGrowsWhenOutnumbered 验证池在消费者多于空闲缓冲时会增长，
// 而不是覆盖还没读完的帧（那会是数据竞争）。
func TestBufferPoolGrowsWhenOutnumbered(t *testing.T) {
	pool := NewBufferPool(1920, 1080)
	held := make([]*Buffer, 0, 4)
	for i := 0; i < 4; i++ {
		held = append(held, pool.Acquire())
	}
	// 4 块同时被占，指针必须互不相同。
	ptrs := map[*image.RGBA]bool{}
	for _, b := range held {
		if ptrs[b.Image()] {
			t.Fatal("池返回了同一块缓冲给两个持有者，会造成数据竞争")
		}
		ptrs[b.Image()] = true
	}
	st := pool.Stats()
	if st.Allocs != 4 {
		t.Fatalf("持有 4 块时分配 %d 块", st.Allocs)
	}
	if st.Peak != 4 {
		t.Fatalf("峰值 %d，应当 4", st.Peak)
	}
	for _, b := range held {
		b.Release()
	}
	if st := pool.Stats(); st.Free != 4 || st.Live != 0 {
		t.Fatalf("释放后 free=%d live=%d", st.Free, st.Live)
	}
}

// BenchmarkCaptureBufferSteadyState 量零拷贝路径的稳态代价：
// 从池里取缓冲 + 写入，不再分配 1080p 图。
func BenchmarkCaptureBufferSteadyState(b *testing.B) {
	pool := NewBufferPool(1920, 1080)
	src := make([]byte, 1920*1080*4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Acquire()
		convertBGRA(buf.Image(), src, 1920, 1080)
		buf.Release()
	}
}

// BenchmarkCaptureAllocPerFrame 是对照组：每帧新建 1080p 图。
func BenchmarkCaptureAllocPerFrame(b *testing.B) {
	src := make([]byte, 1920*1080*4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
		convertBGRA(dst, src, 1920, 1080)
	}
}
