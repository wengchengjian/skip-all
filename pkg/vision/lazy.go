package vision

import (
	"errors"
	"image"
	"sync"
	"time"
)

// LazyOCR 延迟到第一次真正 OCR 时才加载模型，并在空闲后释放模型。
type LazyOCR struct {
	mu      sync.Mutex
	factory func() (OCR, error)
	inner   OCR
	err     error
	timer   *time.Timer
	idle    time.Duration
}

// NewLazyOCR 创建一个按需初始化的 OCR。
func NewLazyOCR(factory func() (OCR, error)) *LazyOCR {
	return &LazyOCR{factory: factory, idle: 15 * time.Second}
}

func (l *LazyOCR) ensureLocked() (OCR, error) {
	if l.err != nil {
		return nil, l.err
	}
	if l.inner != nil {
		return l.inner, nil
	}
	if l.factory == nil {
		l.err = errors.New("OCR 工厂未配置")
		return nil, l.err
	}
	inner, err := l.factory()
	if err != nil {
		l.err = err
		return nil, err
	}
	l.inner = inner
	return inner, nil
}

func (l *LazyOCR) scheduleCloseLocked() {
	if l.idle <= 0 {
		return
	}
	if l.timer != nil {
		l.timer.Stop()
	}
	l.timer = time.AfterFunc(l.idle, l.closeIdle)
}

func (l *LazyOCR) closeIdle() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inner == nil {
		return
	}
	_ = l.inner.Close()
	l.inner = nil
	l.timer = nil
}

func (l *LazyOCR) Recognize(img image.Image, opts OCROptions) ([]TextBlock, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	inner, err := l.ensureLocked()
	if err != nil {
		return nil, err
	}
	blocks, err := inner.Recognize(img, opts)
	l.scheduleCloseLocked()
	return blocks, err
}

func (l *LazyOCR) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
	if l.inner == nil {
		return nil
	}
	err := l.inner.Close()
	l.inner = nil
	return err
}
