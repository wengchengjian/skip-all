package vision

import (
	"errors"
	"image"
	"sync"
)

// LazyOCR 延迟到第一次真正 OCR 时才加载模型，避免启动时常驻大量内存。
type LazyOCR struct {
	mu      sync.Mutex
	factory func() (OCR, error)
	inner   OCR
	err     error
}

// NewLazyOCR 创建一个按需初始化的 OCR。
func NewLazyOCR(factory func() (OCR, error)) *LazyOCR {
	return &LazyOCR{factory: factory}
}

func (l *LazyOCR) ensure() (OCR, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
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

func (l *LazyOCR) Recognize(img image.Image, opts OCROptions) ([]TextBlock, error) {
	inner, err := l.ensure()
	if err != nil {
		return nil, err
	}
	return inner.Recognize(img, opts)
}

func (l *LazyOCR) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inner == nil {
		return nil
	}
	err := l.inner.Close()
	l.inner = nil
	return err
}
