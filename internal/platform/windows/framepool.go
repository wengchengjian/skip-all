//go:build windows

package windows

import (
	"image"
	"sync"
	"sync/atomic"
)

// Buffer 是一块可复用的 1920x1080 RGBA 帧缓冲，按引用计数归还。
//
// 异步识别要读帧，而截图器每拍都会覆写缓冲，所以帧必须能跨拍存活。
// 引用计数让"本拍用完后归还"和"worker 还在读"这两件事解耦：
// 每个使用者 Retain，用完 Release，最后一个释放的把它还给池。
// 这样 worker 直接读原像素，不需要为它深拷贝一张 1080p（约 8MB）。
type Buffer struct {
	img  *image.RGBA
	refs atomic.Int32
	pool *BufferPool
}

// Image 返回缓冲的图像。调用者必须持有引用（Acquire/Retain 之后）。
func (b *Buffer) Image() *image.RGBA { return b.img }

// Retain 增加一个引用，交给另一个 goroutine 使用时调用。
func (b *Buffer) Retain() { b.refs.Add(1) }

// Release 释放一个引用；归零时把缓冲还给池。
func (b *Buffer) Release() {
	if b.refs.Add(-1) <= 0 {
		b.pool.put(b)
	}
}

// BufferPool 管理可复用的 1920x1080 帧缓冲。
//
// 池不设上限：需要多少就分配多少，空闲的留着复用。
// 上限设死会在"识别慢于截图"时逼出一种坏选择——要么等（阻塞）、
// 要么覆盖还没读完的帧（数据竞争）。让它增长，同时用 Stats 观测峰值。
type BufferPool struct {
	mu   sync.Mutex
	free []*Buffer
	// live 是当前被占用的缓冲数，peak 是历史峰值。
	live int
	peak int
	// allocs 是累计分配次数，用来验证稳态零分配。
	allocs int64
	w, h   int
}

// NewBufferPool 创建帧缓冲池，尺寸固定为标准分辨率。
func NewBufferPool(width, height int) *BufferPool {
	if width <= 0 || height <= 0 {
		width, height = 1920, 1080
	}
	return &BufferPool{w: width, h: height}
}

// Acquire 取一块缓冲，引用计数为 1。调用方必须 Release。
func (p *BufferPool) Acquire() *Buffer {
	p.mu.Lock()
	if n := len(p.free); n > 0 {
		b := p.free[n-1]
		p.free = p.free[:n-1]
		p.live++
		if p.live > p.peak {
			p.peak = p.live
		}
		p.mu.Unlock()
		b.refs.Store(1)
		return b
	}
	p.mu.Unlock()

	// 池空：分配一块新的。这发生在识别慢于截图时，属于预期但值得观测。
	b := &Buffer{img: image.NewRGBA(image.Rect(0, 0, p.w, p.h)), pool: p}
	b.refs.Store(1)
	p.mu.Lock()
	p.live++
	if p.live > p.peak {
		p.peak = p.live
	}
	p.allocs++
	p.mu.Unlock()
	return b
}

func (p *BufferPool) put(b *Buffer) {
	p.mu.Lock()
	p.live--
	p.free = append(p.free, b)
	p.mu.Unlock()
}

// BufferStats 是帧缓冲池的观测数据。
type BufferStats struct {
	// Live 是当前占用数，Peak 是历史峰值。
	Live int
	Peak int
	// Allocs 是累计分配次数；稳态下应当不再增长（全部复用）。
	Allocs int64
	// Free 是空闲可复用数量。
	Free int
}

// Stats 返回池的观测数据。
func (p *BufferPool) Stats() BufferStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return BufferStats{Live: p.live, Peak: p.peak, Allocs: p.allocs, Free: len(p.free)}
}
