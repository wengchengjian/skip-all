package vision

import (
	"image"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// OptionButtonOptions 是“描边选项按钮”的形态约束。零值字段使用默认值。
type OptionButtonOptions struct {
	ColorRange [2][3]uint8 // 高亮颜色范围，默认白色
	MinWidth   int         // 按钮最窄，默认 120
	MaxWidth   int         // 按钮最宽，默认 800
	MinHeight  int         // 按钮最矮，默认 28
	MaxHeight  int         // 按钮最高，默认 90
	MinBorder  float64     // 描边行的连续白段至少占按钮宽度的比例，默认 0.6
	MinSide    float64     // 左右描边至少覆盖按钮高度的比例，默认 0.5
	MaxFill    float64     // 按钮内部白像素占比上限（里面只有文字），默认 0.3
	MinText    int         // 按钮内部至少有多少白像素（选项文字），默认 12
}

func (o OptionButtonOptions) withDefaults() OptionButtonOptions {
	if o.ColorRange == [2][3]uint8{} {
		o.ColorRange = [2][3]uint8{{220, 220, 220}, {255, 255, 255}}
	}
	if o.MinWidth <= 0 {
		o.MinWidth = 120
	}
	if o.MaxWidth <= 0 {
		o.MaxWidth = 800
	}
	if o.MinHeight <= 0 {
		o.MinHeight = 28
	}
	if o.MaxHeight <= 0 {
		o.MaxHeight = 90
	}
	if o.MinBorder <= 0 {
		o.MinBorder = 0.6
	}
	if o.MinSide <= 0 {
		o.MinSide = 0.5
	}
	if o.MaxFill <= 0 {
		o.MaxFill = 0.3
	}
	if o.MinText <= 0 {
		o.MinText = 12
	}
	return o
}

// optionRunGap 是连续白段允许的横向断点，只用来吃掉描边上的抗锯齿缝隙。
const optionRunGap = 2

// optionRowGap 是同一枚按钮上下行之间允许的空隙：圆角处的描边会细到没有像素，
// 行与行之间留几行空是正常的。
const optionRowGap = 6

// optionSideWindow 是左右描边搜索的横向余量：圆角按钮的最上一条描边比左右描边更靠内，
// 直接按描边行的端点找侧边会差出一个圆角半径。
const optionSideWindow = 12

// bandRow 是一条白色行投影记录。
type bandRow struct {
	y, count, runStart, runEnd int
}

func (r bandRow) run() int { return r.runEnd - r.runStart + 1 }

// FindOptionButtons 找出区域内“上下两条长描边、左侧一条短描边、中间只有文字”的选项按钮，
// 返回按钮外接矩形（按描边定位，不含紧贴左侧的按键提示）。右侧描边允许被渐变吃掉。
//
// 剧情选项（鸣潮过场里那种「（睁开眼睛）」）就是这种空心描边框；白底实心按钮（面板里的
// “跳过剧情”）、过场特效亮斑、单行字幕都缺一条边或把内部填满，因此不会被认成选项。
func FindOptionButtons(img *image.RGBA, rect geom.Rect, opts OptionButtonOptions) []geom.Rect {
	if img == nil {
		return nil
	}
	opts = opts.withDefaults()
	bounds := img.Bounds()
	r := image.Rect(rect.X1, rect.Y1, rect.X2, rect.Y2).Intersect(bounds)
	if r.Empty() {
		return nil
	}
	lo, hi := opts.ColorRange[0], opts.ColorRange[1]
	hit := func(x, y int) bool {
		c := img.RGBAAt(x, y)
		return c.R >= lo[0] && c.R <= hi[0] && c.G >= lo[1] && c.G <= hi[1] && c.B >= lo[2] && c.B <= hi[2]
	}

	var out []geom.Rect
	var band []bandRow
	gap := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		count, runStart, runEnd, curStart, curEnd, last := 0, -1, -2, -1, -2, -99
		for x := r.Min.X; x < r.Max.X; x++ {
			if !hit(x, y) {
				continue
			}
			count++
			// 连续白段允许几个像素的抗锯齿断点。
			if curStart < 0 || x-last > optionRunGap {
				if curEnd-curStart > runEnd-runStart {
					runStart, runEnd = curStart, curEnd
				}
				curStart = x
			}
			curEnd, last = x, x
		}
		if curEnd-curStart > runEnd-runStart {
			runStart, runEnd = curStart, curEnd
		}
		if count >= 2 {
			band = append(band, bandRow{y: y, count: count, runStart: runStart, runEnd: runEnd})
			gap = 0
			continue
		}
		gap++
		if len(band) > 0 && gap > optionRowGap {
			out = appendOptionButtons(out, band, opts, hit)
			band = band[:0]
		}
	}
	if len(band) > 0 {
		out = appendOptionButtons(out, band, opts, hit)
	}
	return out
}

// appendOptionButtons 检查一组相邻的白色行，把其中成形的描边按钮追加到 out。
func appendOptionButtons(out []geom.Rect, band []bandRow, opts OptionButtonOptions, hit func(x, y int) bool) []geom.Rect {
	if len(band) < 2 {
		return out
	}
	widest := 0
	for _, row := range band {
		if row.run() > widest {
			widest = row.run()
		}
	}
	// 只有横贯按钮的描边行才有这么长的连续白段；文字行、侧边行都短得多。
	minRun := int(opts.MinBorder * float64(widest))
	if minRun < opts.MinWidth {
		minRun = opts.MinWidth
	}
	var edges []bandRow
	for _, row := range band {
		if row.run() >= minRun {
			edges = append(edges, row)
		}
	}
	// 描边可能有 2~3 像素厚，所以每条上描边配的是"能凑够按钮高度的最后一条下描边"，
	// 配完从下描边之后继续找，一列上下叠放的选项就能各认一枚。
	for i := 0; i < len(edges); {
		match := -1
		for k := i + 1; k < len(edges) && edges[k].y-edges[i].y <= opts.MaxHeight; k++ {
			if edges[k].y-edges[i].y+1 >= opts.MinHeight {
				match = k
			}
		}
		if match < 0 {
			i++
			continue
		}
		if rect, ok := optionButton(edges[i], edges[match], opts, hit); ok {
			out = append(out, rect)
			i = match + 1
			continue
		}
		i++
	}
	return out
}

// optionButton 用上下两条描边行加左侧描边还原一枚空心按钮（右侧描边容许渐隐）。
func optionButton(top, bottom bandRow, opts OptionButtonOptions, hit func(x, y int) bool) (geom.Rect, bool) {
	height := bottom.y - top.y + 1
	if height < opts.MinHeight || height > opts.MaxHeight {
		return geom.Rect{}, false
	}
	// 两条描边的公共横贯范围就是按钮的横向范围。
	x1, x2 := max(top.runStart, bottom.runStart), min(top.runEnd, bottom.runEnd)
	if x2-x1+1 < opts.MinWidth/2 {
		return geom.Rect{}, false
	}
	// 左边一定有描边（圆角内收几像素，所以往外找）；右边可能被渐变吃掉，
	// 找不到就以下描边的右端为准——鸣潮这枚选项按钮右侧就是渐隐的。
	left, leftHits := longestColumn(hit, x1-optionSideWindow, x1+2, top.y, bottom.y, false)
	if left < 0 || leftHits < int(opts.MinSide*float64(height)) {
		return geom.Rect{}, false
	}
	right := max(top.runEnd, bottom.runEnd)
	if col, hits := longestColumn(hit, right-optionSideWindow, right+optionSideWindow, top.y, bottom.y, true); col > left && hits >= int(opts.MinSide*float64(height)) {
		right = col
	}
	if right-left < opts.MinWidth || right-left > opts.MaxWidth {
		return geom.Rect{}, false
	}
	// 内部只该有文字：太满说明是实心按钮或亮斑，太空说明是个空框。
	inner := geom.Rect{X1: left + 6, Y1: top.y + 3, X2: right - 5, Y2: bottom.y - 2}
	if inner.Empty() || inner.Width() < 40 {
		return geom.Rect{}, false
	}
	inside := 0
	for y := inner.Y1; y < inner.Y2; y++ {
		for x := inner.X1; x < inner.X2; x++ {
			if hit(x, y) {
				inside++
			}
		}
	}
	if inside < opts.MinText || float64(inside) > opts.MaxFill*float64(inner.Width()*inner.Height()) {
		return geom.Rect{}, false
	}
	return geom.Rect{X1: left, Y1: top.y, X2: right + 1, Y2: bottom.y + 1}, true
}

// longestColumn 在 [x1,x2] 里找 [y1,y2] 范围内白像素最多的一列：outward 为假取最靠左的一条
// （左描边取外沿），为真取最靠右的一条（右描边取外沿）。一条都没有命中时返回 -1。
func longestColumn(hit func(x, y int) bool, x1, x2, y1, y2 int, outward bool) (int, int) {
	best, bestHits := -1, 0
	for x := x1; x <= x2; x++ {
		hits := 0
		for y := y1; y <= y2; y++ {
			if hit(x, y) {
				hits++
			}
		}
		if hits == 0 {
			continue
		}
		if hits > bestHits || (hits == bestHits && ((outward && x > best) || (!outward && x < best))) {
			best, bestHits = x, hits
		}
	}
	return best, bestHits
}
