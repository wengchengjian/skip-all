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
| 连携技：连携条 (813,889)-(1098,890) 的 HSV 金色线 + 左右 连携技-1/2 区域认人 | `zzzChainExt`：连携条金色占比 >=50% 判为连携选择；两边各用头像模板认人，按定位（主C 3 / 副C 2 / 辅助治疗 1）选边；认不出或同分才退回比亮度 | 上游用 HSV+轮廓周长，这里用 RGB 近似；认人用 `avatar_1_*` 71 套模板 |
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

生成结果落在 `games/wuwa/agent_plans.go`（55 个角色）与 `games/zzz/agent_plans.go`（3 个代理人：艾莲/莱卡恩/莱特）。
运行时由 `combat.Spec.PlansByAgent` + `combat.Team.Agent()` 选择：认得出当前角色就用专属连招，认不出退回默认连招。
注意这是**近似序列**——上游真实的 `perform()` / 状态机还带冷却、能量、招架支援、连携判断，本项目没有对应能力。

## 技能可用性

| 游戏 | 上游判据 | 本项目 |
| --- | --- | --- |
| 鸣潮 | `available()` → `box_highlighted()`：技能按钮区域里 `text_white_color`（244-255，`src/__init__.py`）像素占比 > 0 即可用（另外还有 CD 的 OCR 判断） | `combat.Spec.Ready`：共鸣技能/声骸/共鸣解放三个按钮区域（上游 `box_resonance` / `box_echo` / `box_liberation` 坐标折半），纯白占比 >=0.2%，不用就跳过该动作 |
| 绝区零 | 按角色属性分别判定特殊技/终结技（`agent.py` 的 `SPECIAL_*` / `ULTIMATE_*` 色域） | 统一按按钮亮度近似：特殊攻击 (1620,917)-(1720,1017)、终结技 (1728,778)-(1826,876)，亮起占比 >=5% 才放 |

没配判据的动作照旧放（不会因为缺配置停摆）。

冷却另外补了一层：`combat.Spec.Cooldowns` 会 OCR 技能按钮上的剩余秒数（鸣潮 `get_cd` / 绝区零 `has_cd` 的同款判断），读到正数就跳过该动作，结果按 500ms 缓存。

## 切人打分

| 游戏 | 上游 | 本项目 |
| --- | --- | --- |
| 鸣潮 | `get_switch_priority()` 返回 0/100/200/300/400，`_choose_switch_target` 取最高分，再按 intro / buff 时间平手判定；`_unbuffed_support_target()` 优先挑"增益已掉"的治疗与副C；治疗位有 `HEALER_FULL_CON_SWITCH_LOCKOUT = 16` 秒的换入冷却 | `combat.SwitchPolicy` + `Spec.Roles`：按角色定位打分，再给"很久没出场"的槽位、以及**增益已过期**的治疗/副C/辅助加分；治疗位有 16 秒换入冷却（`SwitchPolicy.HealerLockout`）。增益时长 `Spec.BuffTimes` 取自上游 `CharFactory` 的 `buff_time`（未配置时按 主C 14s / 副C 14s / 治疗 28s）；另有 `Spec.SwitchSynergy` 承载各角色 `get_switch_priority()` 里 "在场是某人时必须切我"（MUST 级）的协同对 |
| 绝区零 | `_choose_switch_target` 按 switch_rule_1/2/3 + `_unbuffed_support_target` + `_oldest_switch_target` | 同一套打分模型（共用 `pkg/assist/combat`） |

另：上游 Brant/Encore/Lupa 在 `still_in_liberation()` 时对候选角色返回 MUST；本项目用 `Spec.LiberationMustSwitch` 给仍在解放窗口的候选角色 +1000 分，复现这条切人优先级。Camellya/Jinhsi 的 `has_intro -> MUST` 用 `Spec.SwitchIntroMUST` 复现；Brant 的 `perform_anchor<4s`、Encore 的 `last_heavy<4.6s`、Phrolova 的 `last_liberation<24s`、Douling 的 `last_perform<8s` 这类 NO 锁分别用 `SwitchNoAfterSkill/SwitchNoAfterHeavy/SwitchNoAfterLiberation/SwitchNoAfterPerform` 近似。

角色定位表来自上游：鸣潮 `_char_dict_raw` 的 `char_type`（57 个），绝区零 `agent.py` 的 `AgentTypeEnum`（54 个），分别在 `games/wuwa/agent_roles.go`、`games/zzz/agent_roles.go`。
认不出定位时退回"按顺序切下一个"，不会停摆。

鸣潮角色表覆盖上游 CharFactory 全部 **57 个角色**；其中 9 个用元组键注册（如 `(Labels.char_carlotta, Labels.char_carlotta2)`）。早期生成脚本只匹配单标签键，漏了这 9 个角色。现在 `games/wuwa/agent_ids.go` 保存全部头像模板 id 与别名归一表，`wuwaTeam.Agent`/`Slots` 返回 canonical id，规则/连招/定位表只维护一份。

## 入场技与战斗中断

| 上游 | 本项目 |
| --- | --- |
| 鸣潮 `handle_intro()`：切人后先打入场技（多为共鸣技能 + 普攻，条件分支按角色不同） | `combat.Spec.IntroSteps`：切人后插入的一小段（鸣潮配 技能 + 普攻），状态显示 `入场-*`，打完回主连招 |
| 绝区零无入场技概念 | `IntroSteps` 留空 |
| 角色死亡 / 复活弹窗：`CharDeadException` 跳出循环，`close_revive_popup` 处理弹窗 | `combat.Spec.AbortArea` + `AbortTexts`：OCR 到"复活/失败/重新挑战"等文字就返回 `战斗中断` 并暂停出招（2 秒缓存） |

## 切人/脱战时的状态清理

切人之后技能按钮上的冷却数字、头像、入场技都换了人，所以 `noteSwitch()` 会清空冷却缓存与角色缓存并挂上 `IntroSteps`；脱战时同样清理，避免下一场沿用上一场的判断。

绝区零连携里"邦布"只是第四个可选项（上游 `chain_name_list` 里单独判断），本项目的连携扩展按左右两侧亮度选边，不依赖角色身份，所以不受影响。

## 头像识别的开销控制

上游靠 `possible_agents`（按队伍配置裁剪候选）和 `should_check_all_agents` 控制全量识别频率，本项目用同思路：

- 每个战斗会话第一次认人时全量扫模板（鸣潮 63 张 × 3 槽位 = 189 次比对，绝区零 71 × 3 = 213 次）；
- 认出来的角色进白名单，之后只比这几张（鸣潮 3 次、绝区零 ≤5 次），开销降一个量级以上；
- 10 秒后重新全量一次，换队伍也能跟上。

## 后台角色大招就绪

上游 `update_lib_portrait_icon()` 在角色卡标记位（`lib_mark_char_N`）匹配 `lib_ready_*` 元素模板，判断后台角色的解放是否可用，切人时优先让能放大招的人上场。

本项目：`combat.Spec.UltReady` 配三个槽位区域 + 6 个元素模板（`lib_ready_spectro/electric/wind/fire/ice/havoc`，已从上游拷入 `assets/games/wuwa/templates/libready`），命中给该槽位 +15 分；分数可调（`Bonus`）。

## 鸣潮共鸣回路提示

上游 `BaseChar.is_e_forte_full()` / `is_mouse_forte_full()`：在血条下方的回路提示位匹配 `e_forte`、`mouse_forte` 两个图标，出现就说明强化共鸣技能 / 重击可以打。

本项目 `games/wuwa/combat_ext.go` 的 `wuwaForteExt`：同一区域 (1080,950)-(1220,1045) 依次匹配两个图标，E 图标优先按共鸣技能（状态 `回路-E`），否则鼠标图标点普攻（状态 `回路-重击`），500ms 冷却，阈值 0.6。

## 绝区零角色规则（条件→动作）

上游的 `config/auto_battle_state_handler/站场模板-<角色>.yml` 与 `速切模板-<角色>.yml` 是"条件 → 动作"规则表。
本项目用 `games/zzz/state_rules.go`（生成文件）承载提取结果：**54 个代理人、369 条规则**（去重后），条件用上游原样的 states 表达式，动作是按键序列。

提取覆盖 `config/auto_battle_state_handler` 里全部按角色模板：站场模板(23) / 速切模板(59) / 连携模板(12) / 支援攻击模板(9) / 闪A模板(7) / 双反模板(6) / 快速支援模板(2)，并把 `operation_template` 引用解析到 `config/auto_battle_operation`（407 个操作模板）里的实际按键。

运行时由 `zzzStateRuleExt` 执行，抢在通用连招之前，规则按上游文件顺序匹配、先命中先生效。条件里目前能判定的原子：

| 原子 | 本项目判定 |
| --- | --- |
| `[按键可用-特殊攻击]` | 特殊攻击按钮亮起（颜色占比） |
| `[按键可用-终结技]` | 终结技按钮亮起 |
| `[按键可用-普通攻击]` `[按键可用-闪避]` `[按键可用-快速支援]` | 恒定可用 |
| `[<角色>-特殊技可用]` `[<角色>-终结技可用]` | 同按钮亮起判定 |
| `[<角色>-<状态>]`（如 `艾莲-急冻充能`） | 用上游 `agent.py` 的颜色范围 + 连通块数量判定 |
| `[自定义-黄光切人]` `[自定义-红光闪避]` | 用闪避闪光的黄/红颜色占比判定（与 `zzzDodgeExt` 同区域同色域） |
| `[按键-切换角色-下一个/上一个, 0, N]` | 站场头像变化即记为一次切人，按时间窗判定（分不出上/下一个，两者等价） |
| `[后台-N-击破/强攻/异常/支援/防护/命破]` | 用槽位识别出的代理人 + 上游 `AgentTypeEnum` 分组（`agent_types.go`）判定 |
| `[前台-能量]` `[后台-1-能量]` `[后台-2-能量]` `[<角色>-能量]` | 用上游 `agent_state/energy_3_N` 条状模板定位能量条，再量灰度（90-255）填充长度，量程 0-120，按条件给的区间比较 |
| `[自定义-连携换人, 0, N]` | 连携条可见即视为连携换人发生，按时间窗判定 |
| `[自定义-招架支援]` `[自定义-闪避]` | 黄光/红光闪光的出现时间打点，按时间窗判定 |
| `[自定义-快速支援换人]` | 快速支援按钮模板出现的时间打点，按时间窗判定 |
| `[自定义-失衡时间, 0, N]` | **连携条可见 = 敌人失衡**（连携只在失衡时可用），按"最近一次看到连携条"的时间窗判定 |
| `[自定义-动作不打断]` | 规则队列非空（正在打多段动作） |
| `[自定义-终结技被强制释放]` | 终结技按钮可用（上游在 `auto_battle_agent_context.py` 就是按"前台终结技可用"打的标记） |
| 其余 `自定义-*` | 上游也是自定义配置里的内部状态，本项目没有来源，按不成立处理 |

状态检测共提取 **17 条**（覆盖上游 `COLOR_RANGE_CONNECT` / `COLOR_RANGE_EXIST` / `*_LENGTH` 等判定方式；连通块类按连通块数量判，长度类按颜色占比近似）。

当前 **369 条规则里有 303 条的条件可完全判定（82%）**；整队规则 41 条里 26 条可判定（63%）。合计 410 条中 329 条可用（80%）。

`agent.py` 里的 `AgentStateDef`（颜色范围 + 连通块）也已提取成 `agentStateChecks`（8 条），等有需要接上就能判定 `[<角色>-<状态>]`。

## 绝区零闪避与格挡决策（config/dodge）

上游 `config/dodge/` 是"触发器 → 动作"的闪避决策层，一共 7 份配置（闪避 / 智能格挡 / 本-格挡 / 闪A切* / 切换角色-*）。

本项目照 `智能格挡.sample.yml` 实现 `zzzDodgeExt`：

| 触发 | 上游动作 | 本项目 |
| --- | --- | --- |
| 黄光 | 按"支援攻击"定位优先级切后台角色做格挡，再补普攻（优先级：特殊情况 > 击破 > 命破 > 防护 > 异常 > 下一个） | 用 `agentsByType` 分组给后台两位打分（击破5 > 命破4 > 防护3 > 异常2 > 其它1），按最高分的槽位键切人，再补一次普攻 |
| 红光 | 闪避 + 普攻（闪A） | 按闪避键，再补一次普攻 |
| 声音 | 同上 | **未支持**（没有音频输入） |

上游的"等待 0.2 秒"用动作队列实现：换人/闪避后的一次普攻进队列，下一次 Step 出。头像识别复用队伍白名单，不扫全部 71 张。

## 绝区零整队节奏规则（config/auto_battle/专属配队）

上游 `config/auto_battle/专属配队-<角色>.yml` 是整队级配置：管"什么时候换谁、连携怎么接"，与按角色的 `state_handler` 规则互补。

本项目把它提取成 `games/zzz/team_rules.go`（**41 条规则，覆盖艾莲/莱卡恩/莱特**），运行时先跑整队规则、再跑该角色自己的规则。动作里出现"切换角色-下一个/上一个""连携技-左/右"时，用新增的 `combat.StepKey` 直接按键（上游默认键：切换下一个 space、上一个 c、连携左 q、右 e）。

可判定率偏低：41 条里只有 7 条条件能完全判定，主要卡在 `前台-能量`（7 条）和 `自定义-快速支援换人`、`自定义-招架支援` 这些整队内部状态上——这些需要读能量条与支援状态，本项目没有对应识别。

## 绝区零按钮区域来源

`assets/game_data/screen_info/battle.yml` 里给了各按键的检测区域，本项目全部按它取，不再估算：

| 用途 | 上游区域 | 矩形 |
| --- | --- | --- |
| 战斗判据 | `按键-普通攻击` | (1403,916)-(1505,1017) |
| 特殊攻击就绪/冷却 | `按键-特殊攻击` | (1620,917)-(1720,1017) |
| 终结技就绪/冷却 | `按键-终结技` | (1728,778)-(1826,876) |
| 快速支援 | `按键-切换后援` | (1615,776)-(1726,877) |
| 连携条 | `连携条` | (813,889)-(1098,890)，本项目上下各放宽到 884-900 |
| 中断/结算 | `战斗结果-*` 整排 | (520,840)-(1850,1060)，文案取 完成/再来一次/退出/撤退/倒带 |

## 区域对上游的固化测试

两个游戏都加了"区域必须对得上上游"的测试，防止再出现"区域估错、功能静默失效"：

- `games/wuwa/areas_test.go`：协奏环 = 上游 `get_con_box()` 折半 (716,971)-(779,1034)；目标锁定区必须覆盖 `has_target` 模板记录位置 (1434,958)-(1495,1016)；首领血条 = 上游 `box_of_screen(1269/3840,58/2160,2533/3840,200/2160)` 即 (634,29)-(1266,100)。
- `games/zzz/areas_test.go`：特殊攻击 (1620,917)-(1720,1017)、终结技 (1728,778)-(1826,876)、快速支援 (1615,776)-(1726,877)，以及 screen 配置里的判据区 (1403,916)-(1505,1017) 与结算区 (520,840)-(1850,1060)。

这个测试第一次跑就抓到一处遗漏：上一轮改了规则引擎里的支援区域，却漏改 `zzzQuickAssistExt` 自己的区域，导致真正的快速支援还在用错误坐标。

## 规则动作节流

上游每个 handler 有自己的识别间隔（`check_dodge_interval`、`check_chain_interval` 等），本项目在规则引擎里按动作类型给最小重复间隔：

| 动作 | 最小间隔 |
| --- | --- |
| 普攻 | 250ms |
| 特殊技 | 600ms |
| 终结技 | 800ms |
| 闪避 | 300ms |
| 其它按键（切换/连携） | 200ms |

没有这层节流时，像 `[按键可用-终结技]` 这种条件会一直成立，规则会每帧都按一次终结技。

## 已知近似：后台位可用性原子

规则里 `[后台-1-特殊技可用]`、`[后台-2-特殊技可用]`（各 1 条规则用到）在上游是 `COLOR_CHANNEL_MAX_RANGE_EXIST`：在**该后台槽位的能量条**上找 ≥150 灰度的连通块（`connect_cnt=10`）。

本项目目前用 `agentStateChecks` 里的颜色范围在整条能量区（40,940)-(420,1070）上判定，**区域范围比上游宽**：可能把"前台角色亮着"误判成"后台就绪"。影响面是 2 条规则，且这两条规则的其它条件也多不可判定，实际触发概率很低。要精确化需要按槽位定位能量条再量亮度，等有实机画面一起校准。

## 鸣潮共鸣回路门控（is_forte_full）

上游 `BaseChar.is_forte_full()`：在 4K `(2251,1993)-(2311,2016)`（折半 `(1125,996)-(1156,1008)`）里数近白像素（R 244-255 / G 246-255 / B 250-255）占比，**超过 0.08** 视为回路已满。

不同角色对它的用法相反，所以本项目按角色配门控（`games/wuwa/agent_forte.go`）：

| 角色 | 条件 | 含义 |
| --- | --- | --- |
| 忌炎 `char_jiyan` | `not is_forte_full() and resonance_available()` | 回路**未满**才放共鸣技能 |
| 坎特蕾拉 `char_cantarella` | 同上 | 同上 |
| 加布蕾娜 `char_galbrena` | `is_forte_full() and not need_fast_perform()` | 回路**满**才放 |

其余角色两种写法混杂或条件里没有共鸣技能，机械分类不可靠，**不做门控**（保持原行为）。门控只作用于技能步骤，不影响普攻/声骸/解放。

## 切人确认（switch_next_char 的复查）

上游 `switch_next_char` 在按下切换键后会循环检查 `in_team()` 的当前角色下标，没换过去就继续点普攻再重试。

本项目：切人动作发出后记下切走前的槽位，800ms 窗口内每帧复查 `Team.Current`：

- 槽位变了 → 确认成功，清掉待确认状态；
- 300ms 还没变 → 判定输入被吞，补一次普攻（状态 `切人未生效`）并结束本次确认；
- 800ms 超时 → 放弃，回到正常流程。

后台 PostMessage 输入偶发丢失时，这一步能把"以为切了其实没切"的情况暴露并补救。

## 鸣潮重击（按住左键）

上游 `BaseChar.heavy_attack(duration=0.6)` 与 `heavy_click_forte()`：在攻击点按下左键、保持约 0.6 秒再抬起。

本项目为此加了一条完整链路：`combat.StepHeavy` → `assist.ActionClickHold` → `engine.Input.ClickHold` → Windows 层 `postHoldLocked` / `cursorHoldLocked`（后台模式投递 WM_LBUTTONDOWN、等待、WM_LBUTTONUP；前台模式 SendInput 同形）。

上游在 `do_perform()` 里用到重击的角色共 **14 个**（camellya / cantarella / chouyuan / ciaccona / danjin / encore / jingran / jiyan / lumi / lupa / roccia / shorekeeper / yangyang / yinlin），本项目把这 14 个角色的连招里**解放前的那次普攻换成重击**。

⚠ 这里是**近似**：上游的重击是条件触发（回路满、技能进冷却等），我们按“解放前补一次重击”的固定位置放，具体位置需要实机校准。

## 后台大招就绪槽位（鸣潮）

上游 `update_lib_portrait_icon()` 在 `lib_mark_char_1..3` 这些盒子里找 `lib_ready_<元素>` 模板（阈值 0.8）。
模板记录在 4K 的 `(3653,475)`、`(3654,742)`、`(3653,1003)` 三处，说明就是队伍栏三个槽位；折半后是 `(1826,237)`、`(1827,371)`、`(1826,501)`，模板本身只有约 12×13 像素。
本项目的槽位取 `(1800,210)-(1900,265)` / `(1800,340)-(1900,400)` / `(1800,470)-(1900,530)`，完全覆盖这三个图标位置，并由 `TestUltimateSlotsCoverUpstreamIcons` 锁住。

## 逐角色条件规则（鸣潮）

上游 57 个角色的 `do_perform()` 里，条件分支写法各异。本项目用脚本机械提取：**动作行**（`click_resonance/click_echo/click_liberation/heavy_attack/continues_normal_attack/click`）连同它**外层 if 条件的可表达子集**，生成 `games/wuwa/agent_rules.go`：**171 条规则覆盖 54 个角色**，其余 3 个角色回退到上游 `BaseChar.do_perform()` 的保守动作（声骸→解放→技能→重击）。

可表达原子：`技能可用`（上游 `resonance_available`）、`声骸可用`、`解放可用`、`回路满`/`回路未满`（`is_forte_full`）、`协奏满`（`is_con_full`）、`入场中`（`has_intro`）、`延奏=<角色id>`（`check_outro`）、`飞行中`（`flying`）、`解放中`（`still_in_liberation`）、`需要速切`（`need_fast_perform`）、`共鸣二段`（`can_resonance_step2`）。

后五个原子需要**会话状态**（环是否满、是否还在入场窗口、刚切走的队友是谁、是否在空中、是否还在解放动画），扩展自己从像素看不出来，因此加了可选接口 `combat.StateAware`：实现了它的扩展每帧先收到 `combat.CombatState{Agent, ConcertoFull, InIntro, LastOutro, Flying, InLiberation}`。

`flying()` 照抄上游：开战用 `edge_levitator` 模板（阈值 0.65）确认滑翔装置，图标区 `text_white_color`（244-255）白色占比 < 0.1 算在空中。`still_in_liberation()` 也照抄上游：按角色记录最近一次解放时间，基类窗口 12 秒，Encore 9.5 秒、Xiangliyao 25 秒各自重写；本项目按墙钟计时（未扣游戏冻结时间，属已知近似）。

`check_outro()` 在上游是纯记账：取队伍里 `last_switch_time` 最大的队友（即刚放完延奏的人），`has_intro` 为 false 时返回 `'null'`。本项目在切人动作发出时把当前角色记进 `Session.outroAgent`，只有 `InIntro` 窗口内 `延奏=` 原子才成立。上游 10 个 `check_outro()` 分支里，4 个只改循环时长、3 个只设标志位、3 个是多步特殊收尾（Brant/Changli/Carlotta 的 `perform_in_outro`/`do_perform_outro`）。4 个时长分支只影响上游循环，不产生单步动作；其余 6 个动作分支按上游动作顺序抽成 `agentOutroRules` 近似队列（带 forte 层数/while 循环的精确形态仍无法从像素判定）。生成脚本已能映射 `check_outro()`、`flying()`、`still_in_liberation()` 条件；本次重生成后规则共 171 条（含 9 个元组键角色的规则；Chisa/Douling/Hsin/Iuno/Lucilla 这 5 个只转调助手方法的角色也补了近似规则）。

提取不了的（内部计时器、`crisis_time_left`、`nightfall_time_left`、`last_heavy` 等）不计入条件——那条分支会退化成无条件规则，宁可粗略也不伪造精确。生成脚本：`.research/wuwa_rules.py`。
