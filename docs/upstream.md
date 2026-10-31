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

| 上游行为 | 本项目实现 | 说明 |
| --- | --- | --- |
| while find_skip(): 点击左上角跳过图标 | StorySkipAreas 模板 + “模板跳过” | 图标模板从游戏画面自行裁剪，带白像素掩码 |
| wait_until(skip_confirm, 3s) | 点击跳过后的 5s 确认等待窗口 | 超时后回到剧情识别 |
| click_skip_dialog_confirm：勾选“本次登入不再提示”再点确认 | 只点“确认” | 勾选框是可选的免打扰优化，不勾也能跳过；省一次识别 |
| 确认按钮模板匹配 | OCR 精确匹配“确认/確認/Confirm” + 点文字块中心 | 弹窗宽度随文案变化，按钮不在固定坐标 |
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
