package vision

import (
	"image"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// FindColorRectangles 找出画面里落在颜色范围内、尺寸符合、且填充率足够高的实心矩形。
// 这是上游 find_color_rectangles 的等价实现：掩膜 -> 连通块 -> 外接矩形筛选 -> 填充率。
// 上游用它在整帧里找敌人血条（enemy_health_color_red），所以这里不限定区域。
func FindColorRectangles(img *image.RGBA, rect geom.Rect, colorRange [2][3]uint8, minWidth, minHeight, maxHeight int, threshold float64) []geom.Rect {
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
	mask := make([]bool, w*h)
	for y := 0; y < h; y++ {
		row := img.Pix[(r.Min.Y+y-img.Rect.Min.Y)*img.Stride:]
		for x := 0; x < w; x++ {
			px := row[(r.Min.X+x-img.Rect.Min.X)*4:]
			if px[0] >= colorRange[0][0] && px[0] <= colorRange[1][0] &&
				px[1] >= colorRange[0][1] && px[1] <= colorRange[1][1] &&
				px[2] >= colorRange[0][2] && px[2] <= colorRange[1][2] {
				mask[y*w+x] = true
			}
		}
	}

	var out []geom.Rect
	seen := make([]bool, w*h)
	queue := make([]int, 0, 64)
	for start := 0; start < len(mask); start++ {
		if !mask[start] || seen[start] {
			continue
		}
		queue = queue[:0]
		queue = append(queue, start)
		seen[start] = true
		minX, minY, maxX, maxY, count := w, h, -1, -1, 0
		for len(queue) > 0 {
			idx := queue[len(queue)-1]
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
			if cx > 0 && mask[idx-1] && !seen[idx-1] {
				seen[idx-1] = true
				queue = append(queue, idx-1)
			}
			if cx < w-1 && mask[idx+1] && !seen[idx+1] {
				seen[idx+1] = true
				queue = append(queue, idx+1)
			}
			if cy > 0 && mask[idx-w] && !seen[idx-w] {
				seen[idx-w] = true
				queue = append(queue, idx-w)
			}
			if cy < h-1 && mask[idx+w] && !seen[idx+w] {
				seen[idx+w] = true
				queue = append(queue, idx+w)
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
	}
	return out
}
