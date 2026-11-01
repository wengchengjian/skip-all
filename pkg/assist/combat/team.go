package combat

import (
	"github.com/wengchengjian/skip-all/pkg/assist"
)

// Team 是"当前是哪个角色"的判读接口。
// 两家上游的做法不一样：
//   - 鸣潮 ok-ww：槽位固定，用角色卡上的 1/2/3 键位数字模板找，
//     当前角色的数字被高亮卡面挡住，所以"缺数字的那一格"就是当前角色；
//   - 绝区零 OneDragon：前台槽位固定（大头像），前台/后台用 avatar_1_/avatar_2_
//     两套头像模板区分，槽位顺序即切换顺序。
//
// 共享流程只依赖这个接口，具体判读留给游戏实现。
type Team interface {
	// Current 返回当前角色在队伍里的槽位下标（0 起）；ok=false 表示这一帧没识别出来。
	Current(rt Runtime, frame assist.Frame) (index int, ok bool, err error)
	// Agent 返回当前角色的模板标识（用来选角色专属连招）；ok=false 表示没识别出来。
	Agent(rt Runtime, frame assist.Frame) (id string, ok bool, err error)
}
