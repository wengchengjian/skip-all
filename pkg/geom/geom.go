package geom

// Point 是逻辑坐标点。
type Point struct {
	X int
	Y int
}

// Rect 是左上闭、右下开的矩形区域。
type Rect struct {
	X1 int
	Y1 int
	X2 int
	Y2 int
}

func (r Rect) Width() int  { return r.X2 - r.X1 }
func (r Rect) Height() int { return r.Y2 - r.Y1 }

func (r Rect) Center() Point {
	return Point{X: (r.X1 + r.X2) / 2, Y: (r.Y1 + r.Y2) / 2}
}

func (r Rect) Empty() bool { return r.X2 <= r.X1 || r.Y2 <= r.Y1 }

func (r Rect) Contains(p Point) bool {
	return p.X >= r.X1 && p.X < r.X2 && p.Y >= r.Y1 && p.Y < r.Y2
}
