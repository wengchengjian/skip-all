// Package screen 定义游戏画面与区域配置。
package screen

import (
	"fmt"
	"io/fs"
	"path"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// AreaType 是区域匹配方式。
type AreaType string

const (
	// AreaNone 表示仅用于定位、不参与匹配的区域。
	AreaNone AreaType = "none"
	// AreaText 表示需要 OCR 文本匹配的区域。
	AreaText AreaType = "text"
	// AreaTemplate 表示需要模板匹配的区域。
	AreaTemplate AreaType = "template"
)

// IsValid 报告区域类型是否是已知类型。
func (t AreaType) IsValid() bool {
	switch t {
	case AreaNone, AreaText, AreaTemplate:
		return true
	default:
		return false
	}
}

// Area 是一个画面区域。
type Area struct {
	Name                   string       `yaml:"area_name"`
	Type                   AreaType     `yaml:"area_type"`
	Rect                   geom.Rect    `yaml:"-"`
	RectValues             []int        `yaml:"pc_rect"`
	Text                   string       `yaml:"text"`
	LCSPercent             float64      `yaml:"lcs_percent"`
	ColorRange             *[2][3]uint8 `yaml:"color_range"`
	TemplateSubDir         string       `yaml:"template_sub_dir"`
	TemplateID             string       `yaml:"template_id"`
	TemplateMatchThreshold float64      `yaml:"template_match_threshold"`
	// TemplateWhiteMask 为真时，模板匹配前把区域和模板都做白色掩膜
	// （RGB 全在 210-255 内为白，否则为黑），等价于上游的 convert_dialog_icon。
	TemplateWhiteMask bool `yaml:"template_white_mask"`
	// TemplateScales 是模板匹配要尝试的缩放倍数；上游模板按参考分辨率裁剪，
	// 换分辨率时要跟着缩放。留空表示只按 1:1。
	TemplateScales []float64 `yaml:"template_scales"`
	IDMark         bool      `yaml:"id_mark"`
	GamepadKey     string    `yaml:"gamepad_key"`
	GotoList       []string  `yaml:"goto_list"`
}

// IsText 报告区域是否是文本匹配区域。
func (a *Area) IsText() bool { return a != nil && a.Type == AreaText }

// IsTemplate 报告区域是否是模板匹配区域。
func (a *Area) IsTemplate() bool { return a != nil && a.Type == AreaTemplate }

// CanMatch 报告区域是否具备匹配所需的完整条件。
func (a *Area) CanMatch() bool {
	if a == nil {
		return false
	}
	switch a.Type {
	case AreaText:
		return a.Text != ""
	case AreaTemplate:
		return a.TemplateID != ""
	default:
		return false
	}
}

// Screen 是一个命名画面及其区域集合。
type Screen struct {
	ID       string  `yaml:"screen_id"`
	Name     string  `yaml:"screen_name"`
	PCAlt    bool    `yaml:"pc_alt"`
	AreaList []*Area `yaml:"area_list"`

	areas map[string]*Area
}

// Set 是画面集合。
type Set struct {
	screens map[string]*Screen
}

// NewSet 构造空画面集合。
func NewSet() *Set { return &Set{screens: make(map[string]*Screen)} }

// Add 添加画面，同名画面会覆盖。
func (s *Set) Add(sc *Screen) {
	if sc == nil {
		return
	}
	if s.screens == nil {
		s.screens = make(map[string]*Screen)
	}
	sc.normalize()
	if sc.Name != "" {
		s.screens[sc.Name] = sc
	}
	if sc.ID != "" {
		s.screens[sc.ID] = sc
	}
}

// Get 按名称或 ID 查找画面。
func (s *Set) Get(name string) (*Screen, bool) {
	if s == nil {
		return nil, false
	}
	sc, ok := s.screens[name]
	return sc, ok
}

// Screens 返回全部画面（按名称去重，名称升序）。
func (s *Set) Screens() []*Screen {
	if s == nil {
		return nil
	}
	seen := make(map[*Screen]bool)
	out := make([]*Screen, 0, len(s.screens))
	for _, sc := range s.screens {
		if !seen[sc] {
			seen[sc] = true
			out = append(out, sc)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// LoadFS 从 fs 的 dir 目录加载全部 YAML 画面配置。
func LoadFS(fsys fs.FS, dir string) (*Set, error) {
	set := NewSet()
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("读取画面目录 %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := path.Ext(name)
		if ext != ".yml" && ext != ".yaml" {
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("读取画面文件 %s: %w", name, err)
		}
		var sc Screen
		if err := yaml.Unmarshal(data, &sc); err != nil {
			return nil, fmt.Errorf("解析画面文件 %s: %w", name, err)
		}
		if err := validateScreen(&sc); err != nil {
			return nil, fmt.Errorf("校验画面文件 %s: %w", name, err)
		}
		if sc.ID == "" {
			sc.ID = sc.Name
		}
		set.Add(&sc)
	}
	return set, nil
}

// Area 按名称查找区域。
func (s *Screen) Area(name string) (*Area, bool) {
	if s == nil {
		return nil, false
	}
	a, ok := s.areas[name]
	return a, ok
}

// Areas 返回按配置顺序排列的区域。
func (s *Screen) Areas() []*Area {
	if s == nil {
		return nil
	}
	return s.AreaList
}

func (s *Screen) normalize() {
	if s.areas == nil {
		s.areas = make(map[string]*Area)
	}
	for _, a := range s.AreaList {
		if a == nil {
			continue
		}
		if len(a.RectValues) == 4 {
			a.Rect = geom.Rect{X1: a.RectValues[0], Y1: a.RectValues[1], X2: a.RectValues[2], Y2: a.RectValues[3]}
		}
		a.Type = normalizeAreaType(a.Type, a)
		if a.LCSPercent == 0 {
			a.LCSPercent = 0.5
		}
		if a.TemplateMatchThreshold == 0 {
			a.TemplateMatchThreshold = 0.7
		}
		if a.Name != "" {
			s.areas[a.Name] = a
		}
	}
}

// normalizeAreaType 兼容旧配置的 click/ocr 别名，并按字段推断缺省类型。
func normalizeAreaType(t AreaType, a *Area) AreaType {
	switch t {
	case "click":
		return AreaNone
	case "ocr":
		return AreaText
	}
	if t.IsValid() {
		return t
	}
	switch {
	case a.Text != "":
		return AreaText
	case a.TemplateID != "":
		return AreaTemplate
	default:
		return AreaNone
	}
}

func validateScreen(sc *Screen) error {
	if sc.Name == "" {
		return fmt.Errorf("缺少 screen_name")
	}
	names := make(map[string]struct{}, len(sc.AreaList))
	for _, a := range sc.AreaList {
		if a == nil {
			continue
		}
		if a.Name == "" {
			return fmt.Errorf("画面 %s 存在缺少 area_name 的区域", sc.Name)
		}
		if _, ok := names[a.Name]; ok {
			return fmt.Errorf("画面 %s 存在重名区域 %s", sc.Name, a.Name)
		}
		names[a.Name] = struct{}{}
		if len(a.RectValues) != 0 && len(a.RectValues) != 4 {
			return fmt.Errorf("区域 %s 的 pc_rect 需要 4 个整数", a.Name)
		}
	}
	return nil
}
