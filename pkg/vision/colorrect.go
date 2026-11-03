package vision

import (
	"image"
	"sync"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// colorRectScratch 是扫描用的可复用缓冲。
//
// 原先每次调用都 make 两个 w*h 的 bool 数组：全屏 1920x1080 下每次分配 4MB，
// 10Hz 就是 41.8MB/s 的分配量，是整条帧管线最大的内存压力来源。
// 用对象池复用后稳态零分配。
type colorRectScratch struct {
	mask []bool
	// seen 用"代际"标记代替清零：每次调用 gen 自增，等于 gen 才算访问过。
	// 用 uint8 保持与 []bool 相同的内存（全屏 2MB），同时省掉每次 clear。
	// gen 回绕时整体清一次，256 次调用才一次。
	seen  []uint8
	gen   uint8
	queue []int32
}

var colorRectPool = sync.Pool{
	New: func() any { return &colorRectScratch{} },
}

func getColorRectScratch(n int) *colorRectScratch {
	s := colorRectPool.Get().(*colorRectScratch)
	if cap(s.mask) < n {
		s.mask = make([]bool, n)
		s.seen = make([]uint8, n)
		s.gen = 0
	} else {
		s.mask = s.mask[:n]
		s.seen = s.seen[:n]
	}
	s.queue = s.queue[:0]
	return s
}

// nextGen 取得一个新的代际号。回绕时把 seen 整体清零一次，避免旧标记误命中。
func (s *colorRectScratch) nextGen() uint8 {
	s.gen++
	if s.gen == 0 {
		clear(s.seen)
		s.gen = 1
	}
	return s.gen
}

func putColorRectScratch(s *colorRectScratch) {
	colorRectPool.Put(s)
}

// FindColorRectangles 找出画面里落在颜色范围内、尺寸符合、且填充率足够高的实心矩形。
func FindColorRectangles(img *image.RGBA, rect geom.Rect, colorRange [2][3]uint8, minWidth, minHeight, maxHeight int, threshold float64) []geom.Rect {
	return findColorRectangles(img, rect, colorRange, minWidth, minHeight, maxHeight, threshold, 0)
}

// FindColorRectanglesAny 只判断"是否存在"符合条件的矩形，命中即返回。
// 用于 inCombat 这类只需要布尔结论的场景，能省掉后续所有连通域扫描。
func FindColorRectanglesAny(img *image.RGBA, rect geom.Rect, colorRange [2][3]uint8, minWidth, minHeight, maxHeight int, threshold float64) bool {
	return len(findColorRectangles(img, rect, colorRange, minWidth, minHeight, maxHeight, threshold, 1)) > 0
}

// findColorRectangles 是主实现。limit<=0 表示不限个数。
func findColorRectangles(img *image.RGBA, rect geom.Rect, colorRange [2][3]uint8, minWidth, minHeight, maxHeight int, threshold float64, limit int) []geom.Rect {
	if img == nil {
		return nil
	}
	if threshold <= 0 {
		threshold = 0.95
	}
	bounds := img.Bounds()
	r := image.Rect(rect.X1, rect.Y1, rect.X2, rect.Y2).Intersect(bounds)
	if r.Empty() {
		return nil
	}
	w, h := r.Dx(), r.Dy()
	s := getColorRectScratch(w * h)
	defer putColorRectScratch(s)
	mask, seen := s.mask, s.seen
	gen := s.nextGen()

	// 逐行建立颜色掩膜。rowBase 是目标区域左上角在 Pix 里的偏移，
	// 避免每个像素都做一次坐标换算。
	rowBase := (r.Min.Y-img.Rect.Min.Y)*img.Stride + (r.Min.X-img.Rect.Min.X)*4
	lo0, hi0 := colorRange[0][0], colorRange[1][0]
	lo1, hi1 := colorRange[0][1], colorRange[1][1]
	lo2, hi2 := colorRange[0][2], colorRange[1][2]
	for y := 0; y < h; y++ {
		row := img.Pix[rowBase+y*img.Stride:]
		mrow := mask[y*w : (y+1)*w]
		// 每行按 4 字节一像素遍历。两个分支都写，避免为了清旧值再 memset 一遍。
		pi := 0
		for x := 0; x < w; x++ {
			mrow[x] = row[pi] >= lo0 && row[pi] <= hi0 &&
				row[pi+1] >= lo1 && row[pi+1] <= hi1 &&
				row[pi+2] >= lo2 && row[pi+2] <= hi2
			pi += 4
		}
	}

	var out []geom.Rect
	queue := s.queue
	for start := 0; start < len(mask); start++ {
		if !mask[start] || seen[start] == gen {
			continue
		}
		queue = queue[:0]
		queue = append(queue, int32(start))
		seen[start] = gen
		minX, minY, maxX, maxY, count := w, h, -1, -1, 0
		for len(queue) > 0 {
			idx := int(queue[len(queue)-1])
			queue = queue[:len(queue)-1]
			cx, cy := idx%w, idx/w
			count++
			if cx < minX {
				minX = cx
			}
			if cy < minY {
				minY = cy
			}
			if cx > maxX {
				maxX = cx
			}
			if cy > maxY {
				maxY = cy
			}
			if cx > 0 && mask[idx-1] && seen[idx-1] != gen {
				seen[idx-1] = gen
				queue = append(queue, int32(idx-1))
			}
			if cx < w-1 && mask[idx+1] && seen[idx+1] != gen {
				seen[idx+1] = gen
				queue = append(queue, int32(idx+1))
			}
			if cy > 0 && mask[idx-w] && seen[idx-w] != gen {
				seen[idx-w] = gen
				queue = append(queue, int32(idx-w))
			}
			if cy < h-1 && mask[idx+w] && seen[idx+w] != gen {
				seen[idx+w] = gen
				queue = append(queue, int32(idx+w))
			}
		}
		bw, bh := maxX-minX+1, maxY-minY+1
		if bw < minWidth || bh < minHeight {
			continue
		}
		if maxHeight > 0 && bh > maxHeight {
			continue
		}
		if float64(count)/float64(bw*bh) < threshold {
			continue
		}
		out = append(out, geom.Rect{X1: r.Min.X + minX, Y1: r.Min.Y + minY, X2: r.Min.X + maxX + 1, Y2: r.Min.Y + maxY + 1})
		if limit > 0 && len(out) >= limit {
			return out
		}
	}
	s.queue = queue
	return out
}
