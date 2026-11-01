# 上游来源与移植说明

本项目 v0.1 的“剧情/对话/CG 跳过”行为移植自：

- 仓库：https://github.com/OneDragon-Anything/ZenlessZoneZero-OneDragon
- 提交：cc3e80f3eacbbe11df5af06590cff095d95553a6
- 许可证：GPL-3.0

## 文件对照

| 上游文件 | 本项目文件 | 说明 |
| --- | --- | --- |
| src/zzz_od/application/commission_assistant/commission_assistant_app.py | pkg/assist/storyskip + games/zzz | 按行为逐条移植状态机，不复制 Python 代码 |
| src/zzz_od/application/commission_assistant/commission_assistant_config.py | configs/games/zzz.example.yaml | 保留 mode/dialog_option/interval 等语义 |
| assets/game_data/screen_info/commission_assistant.yml | assets/games/zzz/screens/commission_assistant.yml | 区域坐标、颜色范围、模板引用 |
| assets/game_data/screen_info/normal_world_investigation.yml | assets/games/zzz/screens/normal_world_investigation.yml | 仅迁移“勘域-菜单”守卫区域 |
| src/one_dragon/base/controller/pc_game_window.py | internal/platform/windows/window.go | 客户区、ClientToScreen、缩放 |
| src/one_dragon/base/controller/pc_screenshot/gdi_screencapper_base.py | internal/platform/windows/capture_gdi.go | DIB Section、DC 资源管理 |
| src/one_dragon/base/controller/pc_screenshot/print_window_screencapper.py | internal/platform/windows/capture_gdi.go | PrintWindow 截图策略 |
| src/one_dragon/base/controller/pc_screenshot/bitblt_screencapper.py | internal/platform/windows/capture_gdi.go | BitBlt 回退策略 |
| src/one_dragon/base/controller/pc_controller_base.py | internal/platform/windows/input_sendinput.go | 前台点击、后台 PostMessage 点击、窗口激活、坐标换算 |
| commission_assistant_app.py story_mode/_do_dialog_click | pkg/assist/storyskip/session.go | 灰色跳过过滤、右侧/中间选项、黑屏推进、对话正文、5s 菜单静默 |
| src/one_dragon/base/controller/pc_button/xbox_button_controller.py + vgamepad | internal/platform/windows/gamepad_vigem.go | ViGEmBus 虚拟 Xbox 手柄 |
| src/one_dragon/base/matcher/ocr/ocr_service.py | pkg/vision/service.go | 裁剪、颜色过滤、按帧缓存 |
| src/one_dragon/utils/cv2_utils.py 的 is_in_gray_mask / is_colorful | pkg/vision/color.go | 对话框和黑屏判定 |
| src/one_dragon/base/screen/screen_utils.py | pkg/vision/service.go | 区域匹配、LCS 文本匹配 |
| src/one_dragon/base/matcher/template_matcher.py | pkg/vision/template.go | 带 mask 的模板匹配 |
| src/one_dragon/utils/str_utils.py 的 LCS | pkg/vision/lcs.go | 中文 OCR 容错 |
| assets/template/{menu/back,battle/btn_menu,battle/btn_interact,commission_assistant/1,normal_world_investigation/menu} | assets/games/zzz/templates/ | 仅复制跳过流程实际引用的模板 raw.png/mask.png |

## 修改说明

- Python 的 Operation 节点/轮次框架不直接移植；改为 Go 的 Session.Step 状态机，逻辑顺序与关键常量保持一致。
- 只保留 story_skip 相关逻辑；钓鱼、短信、空洞事件、自动战斗、闪避不在 v0.1 范围内。
- 截图/输入从 Python ctypes/pyautogui 改为 Go syscall/SendInput，行为目标是等价的前台键鼠操作。
- 模板图片可能包含游戏界面截图，仅作为画面匹配数据随项目分发；游戏素材权利归原权利人所有。

## 《鸣潮》剧情跳过

《鸣潮》的跳过流程参考了 ok-wuthering-waves 的行为与识别思路，但**没有复制其代码或模板素材**：上游是 AGPL-3.0，与本项目的 GPL-3.0 不兼容，因此只把流程/时序当作参考，用 Go 重新实现。

- 参考仓库：https://github.com/ok-oldking/ok-wuthering-waves（AGPL-3.0）
- 参考文件：src/task/SkipBaseTask.py（find_skip / skip_confirm / try_click_skip）、src/task/SkipDialogTask.py（skip_message）、src/task/BaseWWTask.py 的 click_skip_dialog_confirm、src/Labels.py 的模板命名
- 本项目实现：games/wuwa/story_skip.go + pkg/assist/storyskip

行为对照：

从本机安装的 ok-ww（`F:\Program Files\ok-ww\data\apps\ok-ww\repo`）反向到的代码事实，已按行为对齐：

- `convert_dialog_icon`（BaseWWTask.py:1309）=`cv2.inRange(img,[210,210,210],[255,255,255])`，同时作用于帧和模板，把"发亮图标 vs 任意背景"压成同一张剪影。本项目对应 `pkg/vision.WhiteMask` + `template_white_mask`，用于鸣潮两个跳过图标区域：误命中从 0.72/0.67 降到 0.28/0.49，阈值因此对齐上游的 0.75（新图标）/0.85（对话栏图标）。
- `try_click_skip`（SkipBaseTask.py:43-49）是 `while find_skip(): click`，点完立刻在 3s 内等确认框。本项目在确认等待窗口里加了"补点跳过"，节流 500ms、每轮最多 3 次，避免点击被吞掉时干等满窗口。
- `check_skip`（:51-55）在没点过跳过时也调一次 `skip_confirm` 兜底。本项目实现了这条：新增 `story/confirm_check` 勾选框模板（从实拍弹窗裁出 44x40，自匹配 1.000、60 张截图最高误命中 0.595），`StorySkipSpec.ConfirmCheckArea` 命中勾选框且 OCR 到确认文案才点（2 秒节流），对应上游"按钮+勾选框"双证。实测：确认框已经开着的截图 → 状态 `确认弹窗`，点 (1184,643)。
- 鸣潮保持后台：上游 ok-ww 对鸣潮就是 `interaction=PostMessage`（`configs/devices.json`），说明 posted 鼠标消息是有效的。之前"光标移过去但点不动"的真正原因是本项目光标路径投递时 lParam 写了 0——等于点在客户区左上角；现在两条路径都把坐标写进 lParam，并先发 WM_MOUSEMOVE 再按下/抬起，与上游一致。
- 后台输入按上游的两处细节对齐：① 按键 lParam 必须带扫描码（上游 `make_lparam` 用 `MapVirtualKey` 填 bit16-23），缺扫描码时游戏常常直接忽略按键；② 点击要把客户区坐标换成屏幕坐标、找出真正覆盖该点的子窗口、再换成该窗口的客户区坐标投递（上游 `update_mouse_pos` 的做法）。实测鸣潮只有一层 `UnrealWindow`、没有子窗口，所以仍然投给顶层窗口。
- 上一版自行发明的"按 ESC 打开剧情梗概再点跳过剧情"已删除：上游就是点跳过图标 → 等确认框 → 点确认，没有走 ESC 菜单这条路。
- 上游日志（working/logs/ok-script.log）证实它在同一台机器上就是点这枚图标：left_click skip_dialog (104, 67)，紧接着 skip_dialog_check (852,607)、confirm_btn_hcenter_vcenter (1419,687)。
- 上游 skip_dialog 是"白底浅色图标"的裁剪（均值亮度 236），直接匹配暗场景只有 0.27；把裁剪里"明显不是背景白"的像素做成字形掩膜后，它在两张真实剧情帧上分别是 0.816 / 0.871，落点 (103,63) 与上游日志的 (104,67) 一致。因此这里也用上游资产：story/skip_dialog_okww = skip_dialog_new_3840x2160_136_350.png 原图 + 派生 mask.png，阈值 0.78，作为对话栏的兜底区域。
- 为什么另外还保留两套截图裁出的模板：实测本机安装的 ok-ww 资产（`assets/templates/skip_dialog_new_3840x2160_136_350.png`，1080p 参考尺寸 21×18）在过场帧上 0.933 命中，但在本项目用户客户端的剧情对话栏上，多尺度 0.5–2.5 共 9 档最高只有 0.725/0.763，达不到 0.85 阈值——图标在两个客户端渲染尺寸不同（21×18 vs 47×53 / 51×53）。上游的模板本身也是它自己客户端的截图，所以这里同样用用户客户端的截图裁模板：`story/skip` 按上游裁剪框从过场帧裁，`story/skip_dialog` / `story/skip_dialog_wide` 从两张真实剧情截图裁。
- 同一枚"六边形 + ▶|"图标在两次真实截图里分别是 47×53 和 51×53（窗口尺寸/缩放略有差异），所以保留两个模板（skip_dialog / skip_dialog_wide）各配一个窗口。本项目不做多尺度匹配：帧统一归一化到 1920×1080，模板按同一基准裁剪，1:1 比较；模板尺寸不对就从真机截图重裁。
- 对话栏（剧情对话）那枚跳过图标是"六边形 + ▶|"：上游记录的位置 (90,52) 对应本项目从真实剧情截图裁出的 51×53 模板，命中点 (75,33)-(126,86)、点击 (100,59)；过场剧情里同一图标的另一套位置/尺寸是 21×18、命中点 (68,175)、点击 (78,184)。两套都要留，因为两个界面的图标尺寸不同。
- 搜索窗按上游的 variance 收窄：`skip_dialog_new` 的裁剪框是 4K(136,350,42,37) → 1080p(68,175,21,18)，上游只在记录位置 ±0.002~0.02 帧宽内搜索；本项目之前用 126×104 的大窗口，会在长相相近的元素上误命中（点了没反应），现在窗口收到 41×38，模板也按上游裁剪框重建（自匹配 1.000，命中点 (68,175)，点击 (78,184)）。
- 字幕式对话没有灰底，鸣潮的门控分三层：① 必须真有剧情 UI 模板命中（跳过图标；菜单按钮只在配置了菜单区域时才算）——`storyMenuVisible` 在"没配置菜单区域"时恒为真是给绝区零留的默认值，不能用于字幕式对话；② 字幕行形态过滤（一行高 8-28 像素、宽 40-900），菜单底部整块 UI（高 32+）不算；③ 确认弹窗要勾选框 + 确认文案双证。109 张真实截图（大世界/菜单/战斗）逐帧跑，触发 0 帧。
- `skip_confirm`（:21-35）的顺序：`click_skip_dialog_confirm`（勾选框 + 确认按钮双证）→ `skip_quest_confirm` → `skip_quest_confirm_new` → 已在队伍即视为完成。本项目用 OCR 精确匹配确认文案覆盖后两种按钮，不勾选"本次登入不再提示"。

| 上游行为 | 本项目实现 | 说明 |
| --- | --- | --- |
| while find_skip(): 点击左上角跳过图标 | StorySkipAreas 模板 + “模板跳过” | 图标模板从游戏画面自行裁剪，带白像素掩码 |
| wait_until(skip_confirm, 3s) | 点击跳过后的 5s 确认等待窗口 | 超时后回到剧情识别 |
| click_skip_dialog_confirm：勾选“本次登入不再提示”再点确认 | 只点“确认” | 勾选框是可选的免打扰优化，不勾也能跳过；省一次识别 |
| 确认按钮模板匹配 | OCR 精确匹配“确认/確認/Confirm” + 点文字块中心 | 弹窗宽度随文案变化，按钮不在固定坐标 |
| （无：上游没有这种面板） | PanelSkipArea：检测到"剧情梗概"这类面板里的"跳过剧情"按钮就点 | 先用白色按钮块判面板在不在（便宜），命中才 OCR 找"跳过剧情"，避免大世界每秒跑 OCR |
| in_team_and_world 作为完成条件 | 无对应判定 | 本项目只做“没有剧情画面就停下”，不判断是否回到队伍 |
| message/message_dialog 点击推进普通对话 | DialogClickArea 点字幕区域 | 鸣潮对话没有灰底对话框，改用白色字幕行判断 |

当前限制：

- 只验过鼠标/后台模式（模板点“跳过”图标 → OCR 点“确认”）。手柄的跳过按键排布没有实测，因此 IgnoreGamepad 打开，全局手柄设置对鸣潮不生效；验证过按键后再打开。
- 跳过图标模板来自过场剧情画面（左上角 ▶| 图标）。普通任务对话界面若图标样式不同，需要再补一个模板。
- 右侧选项区域按“右侧选项/聊天面板”估计，未在真实选项画面校过；误点时改 screens/story.yml 的 pc_rect 即可。

从本机 ok-ww（`F:\Program Files\ok-ww\data\apps\ok-ww\repo\assets`）逆向到的界面事实，用于校订坐标：

- 剧情对话栏：“跳过”在左上角 `skip_dialog` 4K(179,104,55,58) → 1080p 约 (90,52)；同一栏右上角是 `btn_auto_play_dialog` 4K(3381,107,58,46) → 1080p (1690,53)，两者同一高度。
- 过场剧情：“跳过”是另一枚图标 `skip_dialog_new` 4K(136,350,42,37) → 1080p (68,175)，落在画面左侧偏下。
- 确认弹窗至少两种宽度：`confirm_btn` 在 (1390,639,57,77)、`skip_quest_confirm` 在 (1157,734,48,59)；另一份实拍弹窗里确认按钮在 x 1047–1310、y 613–658。所以 OCR 区域取 (950,590)-(1500,730) 覆盖两种。

据此新增了 `story/skip_dialog` 模板：从上面那枚 4K 图标裁剪、缩到 1080p 后做成白色剪影（图标本身是半透明白，直接当模板方差太小），阈值 0.82。用 109 张真实截图做过误命中检查，最高 0.672。

模板来源：assets/games/wuwa/templates/story/skip 从《鸣潮》剧情画面裁剪，按双线性缩放到 1920×1080 逻辑分辨率，掩码只保留图标的白像素，因此不受背景画面影响。游戏素材权利归原权利人所有。

## 自动战斗

| 上游 ok-ww | 本项目 | 说明 |
| --- | --- | --- |
| `AutoCombatTask.run`：`in_team_and_world` 直接返回，否则 `while in_combat(): char.perform()` | `pkg/assist/combat`：检测战斗 -> 按连招序列出招 -> 脱离战斗复位 | 流程只有一份，游戏只给数据 |
| 每角色一个 char 模块（CD OCR、闪避判定、角色专属连招） | 数据驱动的连招序列 `Spec.Plan`；协奏满切人由 `Spec.Concerto` 触发 | 角色专属连招仍未做，等实机确认优先级 |
| 按键：共鸣技能 E、声骸 Q、共鸣解放 R、闪避 左 Shift、切换 1/2/3 | 同上游 `config.py` 的默认值 | 见 `games/wuwa/combat.go` |
| 战斗判据：`check_health_bar()` = `has_health_bar()` 扫敌人血条 + `is_boss()` 扫 Boss 血条 | `pkg/vision.FindColorRectangles` + `Spec.HealthBar` / `Spec.BossBar` | 两个判据都是上游的颜色区间与尺寸参数，整帧扫不需要固定区域 |
| `is_boss()`：区域 (1269/3840, 58/2160)-(2533/3840, 200/2160) 里恰好一条 `boss_health_color`（R245-255/G30-185/B4-75） | `Spec.BossBar{Color, Rect, MinWidth, MinHeight}`，1080p 换算成 (634,29)-(1266,100)，同样要求恰好一条 | 与上游一致 |
| `has_target()`：`has_target` / `no_target` 两张模板在同一区域比分数（阈值 0.6） | `Spec.Target`：帧统一归一化到 1920×1080 后 1:1 比较 | 模板直接取上游 `has_target_3840x2160_2868_1917` / `no_target_3840x2160_2868_1917` |
| `target_enemy()`：没锁定时反复中键锁定敌人 | `Spec.LockInterval` + 上面的目标识别：只在没锁定目标时按中键（新增 `ActionMiddleClick`，走平台层真实中键） | 与上游一致 |
| `switch_healer()`：战前战后切治疗位 | `Spec.HealerKey` + 设置 `switch_healer`：开打第一帧与脱战第一帧各按一次 | 鸣潮已接 `HealerLocator`：按上游 `CharType.HEALER` 的角色模板（维里奈/守岸人/白芷等）认槽位，认不出才退回固定键 |
| `switch_next_char()`：协奏满就换人 | `Spec.Concerto` + `vision.RingCoverage`：头像周围能量环覆盖率达阈值就按切换键 | 环的区域与颜色取自上游 `get_con_box()` 与 `con_colors`；阈值是经验值，需实机校准 |
| `Use Liberation` / `Switch to Healer` 等开关 | `use_liberation` / `use_echo` / `use_dodge` / `interval` / `hold` | 未做切治疗、超时重开等上游特化能力 |

## 绝区零自动战斗

上游是 ZenlessZoneZero-OneDragon（GPL-3.0，与本项目同许可）。

| 上游 | 本项目 | 说明 |
| --- | --- | --- |
| `auto_battle_context.check_battle_state`：`is_normal_attack_btn_available` 决定是否在战斗 | `combat.Spec.DetectAreas` 配 `普通攻击按钮` 区域（(1403,916)-(1505,1017)，模板 `combat/btn_normal_attack`，阈值 0.9） | 模板 raw.png + mask.png 直接取自上游 `assets/template/battle/btn_normal_attack` |
| `_KEY_DEFAULTS`：普通攻击 mouse_left、闪避 shift、特殊攻击 e、终结技 q、角色切换-下一个 space、锁定 mouse_middle | `games/zzz/combat.go` 的 `Keys`：攻击=左键点击、Skill=e、Liberation=q、Dodge=shift、Switch=[space] | 锁定未启用（绝区零默认自动锁定） |
| 每个代理一套 YAML 招式表（`config/auto_battle/*.yml`） | 一套通用连招 `Spec.Plan`：普攻×2 → 特殊技 → 普攻×2 → 闪避；终结技由 `zzzUltimateExt` 在按钮可用时补 | 代理专属招式表未移植 |
| 闪避判定：`auto_battle_dodge_context.check_dodge_flash` 用 YOLO 分类红光/黄光 | `games/zzz/combat_ext.go` 的 `zzzDodgeExt`：屏幕中央黄/红闪光颜色占比超阈值就按闪避键 | 上游是模型，这里是颜色近似；阈值和区域可调 |
| 快速支援：`按键-快速支援` + `btn_assist_1/2` 支援按钮 | `zzzQuickAssistExt`：支援按钮模板命中就按切换键（space） | 模板直接取上游 `assets/template/battle/btn_assist_*` |
| 连携技：连携条 (813,889)-(1098,890) 的 HSV 金色线 + 左右 连携技-1/2 区域 | `zzzChainExt`：连携条金色占比 >=50% 判为连携选择，比左右两侧亮度，亮的一侧按 q/e | 上游用 HSV+轮廓周长，这里用 RGB 近似 + 亮度比较 |
| 终结技：按角色属性判定 `前台-终结技可用` | `zzzUltimateExt`：终结技按钮 (1728,778)-(1826,876) 亮起占比 >=35% 才按 q | 上游按元素色域分别判定，这里统一按钮亮度近似 |
| 游戏专属能力如何接入 | `combat.Spec.Extensions` + `combat.Extension` 接口：通用流程每帧先问扩展，认领就走扩展，否则走连招 | 绝区零专用逻辑全部留在 games/zzz，共享流程不变 |

## 角色与队伍识别（两家上游不一样）

| 项目 | 鸣潮 ok-ww | 绝区零 OneDragon |
| --- | --- | --- |
| 队伍槽位 | 固定区域 `box_char_1/2/3`，用角色头像模板 `find_best_match_in_box` 识别每个位置是谁（`BaseCombatTask.load_chars`） | 四个固定区域：`头像-3-1`(104,40)-(274,110) 前台大图、`头像-3-2`(559,40)-(662,91)、`头像-3-3`(741,40)-(844,91) 后台、`头像-2-2`(594,40)-(697,91) 双人队（`_check_agent_in_parallel`） |
| 当前角色 | 看**角色卡上的 1/2/3 键位数字**：当前角色卡面高亮放大、数字被挡，所以"缺数字的那一格"就是当前角色（`in_team()`） | 看**头像模板族**：前台用 `avatar_1_*`、后台用 `avatar_2_*`，同一个角色前后台各一套模板（`_match_agent_in`） |
| 队伍人数 | `in_team()` 里可见数字个数 + 1 | 3-1/3-2/3-3 都识别到 = 3 人；3-1 + 2-2 = 2 人；只有 3-1 = 1 人 |
| 本项目 | `games/wuwa/team.go` 的 `wuwaTeam`：数字模板缺失定槽位 + 槽位头像模板认人（62 个角色头像取自上游） | `games/zzz/team.go` 的 `zzzTeam`：前台槽位用 `avatar_1_*` 认人（71 套模板已拷入 `templates/avatar1`），后台槽位备好 `avatar_2_*`（`templates/avatar2`） |

共享流程只依赖 `combat.Team` 接口（`Current(rt, frame) (index, ok, err)`）。
鸣潮的协奏满切人现在会按当前槽位选下一个键；识别不出来时退回第一个切换键。

## 角色专属连招

两家上游都不是"数据表"，得从代码/配置里机械推导，本项目生成了一份近似序列：

| 游戏 | 数据来源 | 推导方式 |
| --- | --- | --- |
| 鸣潮 | ok-ww `src/char/<角色>.py` 的 `do_perform()` | 扫描首个出现的 `click_echo / click_liberation / click_resonance / click` 顺序 → 能力之间夹一次普攻 |
| 绝区零 | OneDragon `config/auto_battle/专属配队-<角色>.yml` 与带 `agent_name` 的块 | 统计各按钮出现次数 → 用得多的排前面，能力之间夹一次普攻 |

生成结果落在 `games/wuwa/agent_plans.go`（46 个角色）与 `games/zzz/agent_plans.go`（3 个代理人：艾莲/莱卡恩/莱特）。
运行时由 `combat.Spec.PlansByAgent` + `combat.Team.Agent()` 选择：认得出当前角色就用专属连招，认不出退回默认连招。
注意这是**近似序列**——上游真实的 `perform()` / 状态机还带冷却、能量、招架支援、连携判断，本项目没有对应能力。

## 技能可用性

| 游戏 | 上游判据 | 本项目 |
| --- | --- | --- |
| 鸣潮 | `available()` → `box_highlighted()`：技能按钮区域里 `text_white_color`（253-255）像素占比 > 0 即可用（另外还有 CD 的 OCR 判断） | `combat.Spec.Ready`：共鸣技能/声骸/共鸣解放三个按钮区域（上游 `box_resonance` / `box_echo` / `box_liberation` 坐标折半），纯白占比 >=0.2%，不用就跳过该动作 |
| 绝区零 | 按角色属性分别判定特殊技/终结技（`agent.py` 的 `SPECIAL_*` / `ULTIMATE_*` 色域） | 统一按按钮亮度近似：特殊攻击 (1620,917)-(1720,1017)、终结技 (1728,778)-(1826,876)，亮起占比 >=5% 才放 |

没配判据的动作照旧放（不会因为缺配置停摆）。

冷却另外补了一层：`combat.Spec.Cooldowns` 会 OCR 技能按钮上的剩余秒数（鸣潮 `get_cd` / 绝区零 `has_cd` 的同款判断），读到正数就跳过该动作，结果按 500ms 缓存。

## 切人打分

| 游戏 | 上游 | 本项目 |
| --- | --- | --- |
| 鸣潮 | `get_switch_priority()` 返回 0/100/200/300/400，`_choose_switch_target` 取最高分，再按 intro / buff 时间平手判定 | `combat.SwitchPolicy` + `Spec.Roles`：按角色定位打分（主C打完优先治疗/副C，治疗/辅助打完优先主C），再给"很久没出场"的槽位加分，最高分者胜 |
| 绝区零 | `_choose_switch_target` 按 switch_rule_1/2/3 + `_unbuffed_support_target` + `_oldest_switch_target` | 同一套打分模型（共用 `pkg/assist/combat`） |

角色定位表来自上游：鸣潮 `_char_dict_raw` 的 `char_type`（48 个），绝区零 `agent.py` 的 `AgentTypeEnum`（54 个），分别在 `games/wuwa/agent_roles.go`、`games/zzz/agent_roles.go`。
认不出定位时退回"按顺序切下一个"，不会停摆。

## 入场技与战斗中断

| 上游 | 本项目 |
| --- | --- |
| 鸣潮 `handle_intro()`：切人后先打入场技（多为共鸣技能 + 普攻，条件分支按角色不同） | `combat.Spec.IntroSteps`：切人后插入的一小段（鸣潮配 技能 + 普攻），状态显示 `入场-*`，打完回主连招 |
| 绝区零无入场技概念 | `IntroSteps` 留空 |
| 角色死亡 / 复活弹窗：`CharDeadException` 跳出循环，`close_revive_popup` 处理弹窗 | `combat.Spec.AbortArea` + `AbortTexts`：OCR 到"复活/失败/重新挑战"等文字就返回 `战斗中断` 并暂停出招（2 秒缓存） |

## 切人/脱战时的状态清理

切人之后技能按钮上的冷却数字、头像、入场技都换了人，所以 `noteSwitch()` 会清空冷却缓存与角色缓存并挂上 `IntroSteps`；脱战时同样清理，避免下一场沿用上一场的判断。

绝区零连携里"邦布"只是第四个可选项（上游 `chain_name_list` 里单独判断），本项目的连携扩展按左右两侧亮度选边，不依赖角色身份，所以不受影响。
