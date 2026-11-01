package wuwa

import "github.com/wengchengjian/skip-all/pkg/assist/combat"

// agentForteGate 是"技能要等共鸣回路状态"的角色表，从上游 ok-ww 的
// src/char/<角色>.py 里 is_forte_full() 与共鸣技能的关系机械分类而来：
//   - 条件写成 is_forte_full() 才放共鸣技能 -> full
//   - 条件写成 not is_forte_full() 才放      -> not_full
//
// 两种情况同时出现、或条件里没有共鸣技能的角色不门控（上游语义各不相同，机械分类不可靠）。
var agentForteGate = map[string]combat.ForteNeed{
	"char_jiyan":      combat.ForteNeedNotFull, // 忌炎：回路没满才放共鸣技能
	"char_cantarella": combat.ForteNeedNotFull, // 坎特蕾拉
	"char_galbrena":   combat.ForteNeedFull,    // 加布蕾娜
}
