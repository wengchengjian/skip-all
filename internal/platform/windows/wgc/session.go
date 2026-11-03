//go:build windows

package wgc

import (
	"fmt"
	"time"
)

// Session 是一个窗口的 WGC 捕获会话。
//
// 只能在调用过 RoInitialize 的线程上使用；调用方负责固定线程（见 windows.WGCDevice）。
type Session struct {
	item          comPtr
	dev           *d3dDevice
	ownsDev       bool
	pool          *framePool
	width, height int
}

// OpenSession 为一个窗口建立 WGC 捕获会话，内部创建独立的 D3D11 设备。
func OpenSession(hwnd uintptr, width, height int) (*Session, error) {
	dev, err := newD3DDevice()
	if err != nil {
		return nil, err
	}
	return openSession(dev, true, hwnd, width, height)
}

// OpenSessionWithDevice 用已有设备建立会话，关闭会话不会释放设备。
//
// 窗口改分辨率或句柄变化时只需要新的帧池，设备可以继续用；
// 每建一次会话就新建一个设备会让 WDDM 资源随重建次数累积。
func OpenSessionWithDevice(dev *Device, hwnd uintptr, width, height int) (*Session, error) {
	if dev == nil || dev.dev == nil {
		return nil, fmt.Errorf("D3D 设备为空")
	}
	return openSession(dev.dev, false, hwnd, width, height)
}

func openSession(dev *d3dDevice, ownsDev bool, hwnd uintptr, width, height int) (*Session, error) {
	item, err := captureItem(hwnd)
	if err != nil {
		if ownsDev {
			dev.release()
		}
		return nil, err
	}
	pool, err := newFramePool(dev, item, int32(width), int32(height))
	if err != nil {
		if ownsDev {
			dev.release()
		}
		item.release()
		return nil, err
	}
	return &Session{item: item, dev: dev, ownsDev: ownsDev, pool: pool, width: width, height: height}, nil
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

// Close 释放会话资源；共享的设备由调用方负责关闭。
func (s *Session) Close() {
	if s == nil {
		return
	}
	s.pool.release()
	if s.ownsDev {
		s.dev.release()
	}
	s.item.release()
}
