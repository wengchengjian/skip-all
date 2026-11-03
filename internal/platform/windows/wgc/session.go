//go:build windows

package wgc

import (
	"time"
)

// Session 是一个窗口的 WGC 捕获会话。
//
// 只能在调用过 RoInitialize 的线程上使用；调用方负责固定线程（见 windows.WGCDevice）。
type Session struct {
	item          comPtr
	dev           *d3dDevice
	pool          *framePool
	width, height int
}

// OpenSession 为一个窗口建立 WGC 捕获会话。
//
// width/height 是窗口客户区尺寸，用于建帧池；实际帧尺寸以纹理描述为准。
func OpenSession(hwnd uintptr, width, height int) (*Session, error) {
	item, err := captureItem(hwnd)
	if err != nil {
		return nil, err
	}
	dev, err := newD3DDevice()
	if err != nil {
		item.release()
		return nil, err
	}
	pool, err := newFramePool(dev, item, int32(width), int32(height))
	if err != nil {
		dev.release()
		item.release()
		return nil, err
	}
	return &Session{item: item, dev: dev, pool: pool, width: width, height: height}, nil
}

// Size 返回建会话时的窗口尺寸。
func (s *Session) Size() (int, int) { return s.width, s.height }

// Capture 取当前最新一帧，把 BGRA 像素写进 dst，并返回这一帧的实际尺寸。
//
// 返回的实际尺寸可能和建会话时给的尺寸不同（窗口改过分辨率），
// 调用方必须按返回值去解释 dst，不要用建会话时的尺寸。
func (s *Session) Capture(dst []byte, cropX, cropY, cropW, cropH int, timeout time.Duration) (int, int, error) {
	frame, tex, err := s.pool.nextFrame(timeout)
	if err != nil {
		return 0, 0, err
	}
	defer frame.release()
	defer tex.release()
	return s.dev.copyTo(dst, tex, cropX, cropY, cropW, cropH)
}

// Close 释放会话资源。
func (s *Session) Close() {
	if s == nil {
		return
	}
	s.pool.release()
	s.dev.release()
	s.item.release()
}
