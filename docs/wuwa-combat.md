# 鸣潮自动战斗：能力与缺口

上游权威实现是 [ok-ww](https://github.com/ok-oldking/ok-wuthering-waves)。本文件记录每项能力的上游依据、本项目落点、状态，以及**明确不做**的部分与原因。

## 已移植

| 能力 | 上游依据 | 本项目落点 |
| --- | --- | --- |
| 战斗判据 | `has_health_bar()`：整帧找 `enemy_health_color_red`（174-225 / 55-85 / 55-76），宽≥50、高 5-14 | `games/wuwa/combat.go` 的 `HealthBar` |
| 首领判据 | `is_boss()`：`(1269,58)-(2533,200)`/4K 的 `boss_health_color` 且区域内恰好一条 | `BossBar`（折半 `(634,29)-(1266,100)`） |
| 协奏满切人 | `get_current_con()` 必须等于 1；`_ensure_ring_index()` 先认同色再单色计数 | `Concerto`（门限 0.95）+ `vision.RingCoverages` |
| 切人打分 | `get_switch_priority` / `_choose_switch_target` / `_unbuffed_support_target` | `SwitchPolicy` + `Roles` + `BuffTimes`（57 个角色增益时长）+ `SwitchSynergy`（16 个协同对，含 Phrolova 的 14 秒延后条件）+ `LiberationMustSwitch`（Brant/Encore/Lupa/Zani 在解放窗口内 MUST）+ `SwitchIntroMUST`（Camellya/Jinhsi 入场 MUST）+ `SwitchIntroAfter`（Luhesi 24s、Suisui 40s）+ `SwitchNoAfterHeavy/Skill/Liberation/Perform`（Encore/Brant/Phrolova/Douling/Zani/Suisui 的 NO 锁）+ `ResonanceMustSwitch`（Encore 共鸣二段 MUST） |
| 切人等待 `wait_switch()` | Zani 夜落 2.2s、Carlotta 声骸后 2.5s、Cantarella 重击后 8s、Hiyuki/Augusta 的 check_outro 时长 | `Spec.HoldSwitchAfterHeavy` / `HoldSwitchAfterEcho` / `HoldSwitchAfterOutro` |
| 治疗位换入冷却 | `HEALER_FULL_CON_SWITCH_LOCKOUT = 16` 秒 | `SwitchPolicy.HealerLockout` |
| 切人确认 | `switch_next_char` 里循环复查 `in_team()`，没切过去补普攻 | `Session` 的 800ms 复查窗口 |
| 角色识别 | `in_team()`：队伍栏三个数字标签缺一个即为当前角色 | `wuwaTeam`（模板 69 个） |
| 后台大招就绪 | `update_lib_portrait_icon()`：`lib_mark_char_N` 里找 `lib_ready_<元素>`，阈值 0.8 | `UltReady`（3 槽位、6 模板） |
| 技能就绪 / 冷却 | `available()` = 白像素（244-255）> 0 且无 CD 数字 | `Ready`（3 个按钮）+ `Cooldowns` |
| 共鸣回路提示 | `is_mouse_forte_full()` / `is_e_forte_full()`，阈值 0.6 | `wuwaForteExt`（强化 E / 重击） |
| 回路满判据 | `is_forte_full()`：`(2251,1993)-(2311,2016)`/4K，近白占比 > 0.08 | `ForteFull` + `ForteGate`（3 个角色） |
| 入场等待 | `wait_down()` / `wait_intro()`：点普攻直到技能可用，超时 1.2 秒 | `Spec.Intro` |
| 今汐入场爆发 | `Jinhsi.handle_intro()`：技能能按就按，进冷却后声骸收尾 | `IntroByAgent["char_jinhsi"]` |
| 今汐 incarnate | `Jinhsi.handle_incarnation()`：交替普攻/共鸣，动画结束后收尾 | `wuwaJinhsiExt` |
| 重击 | `heavy_attack(duration=0.6)` / `heavy_click_forte()` | `StepHeavy` → `ActionClickHold` → `Input.ClickHold`（16 个角色用到） |
| 空中判据 | `BaseChar.flying()`：`edge_levitator` 模板确认滑翔装置（阈值 0.65），图标区白色占比 < 0.1 算在空中 | `FlyingCheck` + `CombatState.Flying` |
| 角色连招 | 各 `do_perform()` 里出手顺序 | `agentPlans`（55 个角色） |
| **逐角色条件规则** | 各 `do_perform()` 里的 `if` 分支 | `wuwaRuleExt` + `agent_rules.go`（**171 条角色专属规则（54 个角色）+ BaseChar 回退规则（其余 3 个角色）**），条件语法同绝区零：`! & | ()`，原子 = 技能可用 / 声骸可用 / 解放可用 / 回路满 / 回路未满 / 协奏满 / 入场中 / 飞行中 / 解放中 / 需要速切 / 共鸣二段 / 延奏=<角色id> |
| 速切判定 | `need_fast_perform()`：队友里有人对当前角色返回 MUST | `CombatState.NeedFastPerform` |
| 角色定位 | `char_type` | `agentRoles`（57 个） |

## 明确不做

| 项 | 上游规模 | 为什么不做 |
| --- | --- | --- |
| 逐条件连招里**无法表达**的部分 | 见下 | 条件依赖上游内部计时器（`crisis_time_left`、`nightfall_time_left`、`last_heavy` 等），屏幕上观测不到；这类条件所在的分支**被降级为无条件规则或直接跳过**，不假装精确 |

机械提取时统计到的条件出现次数（来自生成脚本输出；其中 `is_con_full` / `has_intro` / `check_outro` / `flying` / `still_in_liberation` / `need_fast_perform` / `can_resonance_step2` 已映射成规则原子）：

```text
flying 35   is_con_full 11   check_outro 8   has_intro 7   still_in_liberation 4   can_resonance_step2 1
```
> 口径：上面是生成脚本的统计（57 个角色、`do_perform()` 一层内联）；全量 `src/char/*.py` 的 if 行更多（审计：flying 45、check_outro 10、still_in_liberation 8），差异来自跨方法（`do_perform_interlock`、`wait_for_accelerate_ready`）与 `get_switch_priority` 等不在提取范围的分支。

| `check_outro()` 的分支 | 10 个角色 | 检测已做会话记账（`CombatState.LastOutro` + `延奏=` 原子）；Brant/Changli/Carlotta 的多步收尾、Cantarella/Phrolova/Lupa 的标志位分支都已接入近似队列（`agentOutroRules`），Hiyuki×2/Augusta 的 3 个时长分支用 `HoldSwitchAfterOutro` 近似，Linnai 的入场等待不产生单步动作 |
| `get_switch_priority()` 重写 | 26 个角色 | 同上；其中**可机械提取的那一类"在场是谁则必须切我"已经做了**（`SwitchSynergy`，16 对；含 Carlotta<-Zhezhi、Changli<-Brant、Linnai<-Mornye、Mornye<-Aemeath/Qingxiao、ShoreKeeper<-Augusta） |
| 协奏百分比 | — | 我们已有环覆盖率，上游也是面积比，改成百分比不改变任何决策 |

## 未验证的部分

所有阈值都来自上游常量或折半坐标，但**没有一帧实机画面**验证过命中率。参数分档见 [combat-params.md](combat-params.md)，其中"估算"那档需要开游戏校准。
