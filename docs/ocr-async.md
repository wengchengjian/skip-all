# OCR 异步与帧缓冲

本文说明"识别不再阻塞决策拍"的实现方式，以及为什么必须先把帧缓冲池做好。

## 为什么需要异步

识别是同步阻塞的：`Session.Step` 里调 `Runtime.OCR`，`Runtime.OCR` 里调
PP-OCR，一次 120~145ms。引擎的 `stepFeatures` 串行调用每个功能，所以
一次识别直接吃掉整拍。

实测（`internal/engine/sync_budget_test.go`，10Hz 即 100ms 预算）：

| 工作耗时 | 实际周期 | TickLag |
| --- | --- | --- |
| 60ms（预算内） | 100.6ms | 0.3ms |
| 130ms（超预算） | 130.4ms | 30.1ms |
| 250ms（严重超） | 250.4ms | 150.2ms |

结论：**工作耗时低于一拍预算时，固定步长 tick 能把它吸收掉；一旦超过，
周期就等于工作耗时。** 所以异步的价值不在"平时更快"，而在"识别偶尔
变慢时不把整拍拖垮"。

## 三层结构

```
截图（写进池缓冲）
   ↓  assist.Frame{Image, Retain, Release}
决策拍（读上一次的识别结果，立刻返回）
   ↓
worker 池（后台跑 OCR，完成后更新结果快照）
```

### 1. 帧缓冲池（`internal/platform/windows/framepool.go`）

截图器原本每帧 `image.NewRGBA(1920x1080)`，一次分配约 8MB。异步之后更糟：
worker 要读帧，而截图器下一拍就会覆写同一块缓冲。

`Buffer` 按引用计数管理：

- 截图器 `Acquire` 一块，引用计数 1
- 决策拍用完 `Release`
- 每个 worker 读之前 `Retain`，读完 `Release`
- 最后一个释放的把缓冲还给池

这样 worker 直接读原像素，**不需要为每帧深拷贝 8MB**。

池不设上限：需要多少分配多少，空闲的留着复用。设上限会逼出一种坏选择——
要么等（阻塞），要么覆盖还没读完的帧（数据竞争）。用 `Stats()` 观测峰值。

### 2. 异步运行时（`pkg/assist/async_ocr.go`）

`AsyncRuntime` 包在 `Runtime` 外面，只接管 `OCR`：

- **首次识别同步等一次**：某区域从未识别过时没有历史结果可用，
  此时同步算一次，保证首帧不会因为"还没算完"而瞎掉。
- **之后立刻返回上一次结果**，同时把新请求丢给 worker。
- **同一区域最多一个在途请求**：慢识别时后续请求被合并（`Dropped` 计数），
  不会堆成山。
- **结果按帧序号单调前进**：慢 worker 不会用旧结果覆盖新结果。
- **队列满时放弃刷新**：返回旧结果，不阻塞决策。

`MatchTemplate` / `ColorStats` 不异步——它们是纯内存扫描，没有网络/模型开销。

### 3. 配置

```yaml
ocr:
  workers: 2   # 1 = 同步（旧行为）；2~4 = 异步 worker 数
```

上限 4：每个 worker 都要抢 CPU，开太多反而互相拖慢。

## 实测收益

`games/wuwa/combat_async_test.go`（真实 PP-OCR + 模拟对局）：

| 配置 | 每帧 | 最慢帧 |
| --- | --- | --- |
| 同步 OCR | 22.7ms | 88.8ms |
| 异步 OCR（2 worker） | **8.0ms** | **60.9ms** |

## 观测

- `TickLag` / `TickDropped`：UI 上 FPS chip 落后超过 100ms 标橙
- `AsyncStats`：`Submitted` / `Completed` / `Dropped` / `Pending`
  - `Dropped` 持续增长说明识别跟不上调用频率
  - `Pending` 长期为 1 说明 worker 数不够或单次识别太慢

## 验证

```powershell
go test ./pkg/assist -run TestAsyncOCR -race
go test ./internal/platform/windows -run TestBufferPool -race
go test ./games/wuwa -run TestCombatAsyncOCRFrameCost
go test ./internal/engine -run TestSyncOCR
```

## 注意

- **不要**在异步 worker 里直接读 `frame.Image` 而不 `Retain`：
  缓冲随时可能被下一拍覆写。
- **不要**给 `BufferPool` 加容量上限：那会引入"等或覆盖"的两难。
- 首帧同步等待是设计上的例外，不是遗漏。如果要连首帧都不等，
  需要先给该区域一个初始值（比如上一次会话的结果）。
