package engine

import (
	"context"
	"time"
)

// Tick 是一次逻辑节拍。所有功能在同一拍里跑，日志与指标都按 ID 对齐。
type Tick struct {
	// ID 从 1 开始单调递增，一拍一个。
	ID uint64
	// Time 是本拍开始时间。
	Time time.Time
	// Lag 是本拍实际开始时间相对计划时间的落后量。
	Lag time.Duration
}

// tickClock 是固定步长逻辑时钟。
//
// 与"睡 wait 毫秒"的区别：周期从本拍开始时间算，而不是从干完活算，
// 所以截图、识别、动作执行的耗时被吸收进这一拍里，周期不会随工作量漂移。
// 落后超过 maxLagFactor 个周期时直接跳拍（不追赶），避免积压后连发。
type tickClock struct {
	interval time.Duration
	next     time.Time
	id       uint64
	lag      time.Duration
	dropped  uint64
}

// maxLagFactor 是允许的落后倍数：超过就直接跳到当前时间。
const maxLagFactor = 5

// maxBlockingHold 是可以同步阻塞执行的最长按住时长。
// 超过它就走 Press + 定时 Release，让引擎在按住期间继续截图决策。
const maxBlockingHold = 100 * time.Millisecond

func newTickClock(interval time.Duration) *tickClock {
	if interval <= 0 {
		interval = 200 * time.Millisecond
	}
	return &tickClock{interval: interval}
}

// Begin 开始新的一拍，返回本拍信息并记录落后量。
func (c *tickClock) Begin(now time.Time) Tick {
	lag := time.Duration(0)
	if !c.next.IsZero() {
		lag = now.Sub(c.next)
		if lag < 0 {
			lag = 0
		}
		if lag > maxLagFactor*c.interval {
			// 落后太多：跳拍，从当前时间重新对齐，不补发。
			c.dropped++
			lag = 0
		}
	}
	c.id++
	c.lag = lag
	return Tick{ID: c.id, Time: now, Lag: lag}
}

// Schedule 用本拍开始时间和本拍要求的等待时长设定下一拍的计划时间。
func (c *tickClock) Schedule(start time.Time, wait time.Duration) {
	if wait <= 0 {
		wait = c.interval
	}
	c.next = start.Add(wait)
}

// Wait 返回到下一拍计划时间还需要等多久。
func (c *tickClock) Wait(now time.Time) time.Duration {
	d := c.next.Sub(now)
	if d < 0 {
		return 0
	}
	return d
}

// Metrics 返回时钟的观测指标。
func (c *tickClock) Metrics() (tickID uint64, lag time.Duration, dropped uint64) {
	return c.id, c.lag, c.dropped
}

// sleepUntil 睡到指定时刻；ctx 取消时返回 false。
func sleepUntil(ctx context.Context, at time.Time) bool {
	d := time.Until(at)
	if d <= 0 {
		return ctx.Err() == nil
	}
	return sleep(ctx, d)
}
