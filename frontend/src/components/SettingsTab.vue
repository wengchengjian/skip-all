<script setup lang="ts">
import { computed, reactive, watch } from "vue";
import type { ConfigView, GameView, UpdateView } from "../api";

const props = defineProps<{
  config: ConfigView | null;
  games: GameView[];
  selectedGameId: string;
  saving: boolean;
  version: string;
  updateInfo: UpdateView | null;
  updateChecking: boolean;
  updateInstalling: boolean;
  updateDownloading: boolean;
  updateProgress: number;
}>();

const emit = defineEmits<{
  (e: "save", view: ConfigView): void;
  (e: "check-update"): void;
  (e: "download-update"): void;
  (e: "install-update"): void;
}>();

// 功能 ID 来自后端 registry，通过 GameView.features 暴露；设置页只渲染游戏确实拥有的功能。
const STORY_SKIP = "story_skip";
const AUTO_COMBAT = "auto_combat";

const form = reactive({
  mode: "skip",
  dialogOption: "last",
  dialogClickInterval: 0.5,
  optionClickInterval: 0.1,
  sleepAfterEmpty: 0.5,
  pauseWhenBackground: true,
  captureMethod: "auto",
  inputMode: "background",
  cursorMode: "auto",
  ocrGpu: "auto",
  gamepad: false,
  useLiberation: true,
  useEcho: true,
  useDodge: true,
  switchHealer: true,
  dodgeFlash: true,
  quickAssist: true,
  chainAttack: true,
  ultimateReady: true,
  useConcerto: true,
  combatInterval: 0.15,
  excludedGames: [] as string[],
  hotkeyToggle: "Home",
  hotkeyStorySkip: "F11",
  hotkeyAutoCombat: "F12",
  hotkeyGameOnly: true,
  hotkeySound: true,
  updateAutoCheck: true,
  updateAutoDownload: true,
});

function toNumber(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

const selectedGame = computed(() => props.games.find((game) => game.id === props.selectedGameId) ?? null);
const featureIds = computed(() => new Set((selectedGame.value?.features ?? []).map((feature) => feature.id)));
const hasStorySkip = computed(() => featureIds.value.has(STORY_SKIP));
const hasAutoCombat = computed(() => featureIds.value.has(AUTO_COMBAT));
const gameTitle = computed(() => selectedGame.value?.name ?? "未选择游戏");
const storyIndex = computed(() => "01");
const combatIndex = computed(() => (hasStorySkip.value ? "02" : "01"));

watch(
  () => [props.config, props.selectedGameId] as const,
  ([config]) => {
    if (!config) return;
    const feature = config.games?.[props.selectedGameId]?.Features?.[STORY_SKIP];
    const settings = feature?.Settings ?? {};
    form.mode = String(settings.mode ?? "skip");
    form.dialogOption = String(settings.dialog_option ?? "last");
    form.dialogClickInterval = toNumber(settings.dialog_click_interval, 0.5);
    form.optionClickInterval = toNumber(settings.option_click_interval, 0.1);
    form.sleepAfterEmpty = toNumber(settings.sleep_after_empty, 0.5);
    const combat = config.games?.[props.selectedGameId]?.Features?.[AUTO_COMBAT];
    const combatSettings = combat?.Settings ?? {};
    form.useLiberation = combatSettings.use_liberation ?? true;
    form.useEcho = combatSettings.use_echo ?? true;
    form.useDodge = combatSettings.use_dodge ?? true;
    form.switchHealer = combatSettings.switch_healer ?? true;
    form.dodgeFlash = combatSettings.ext_dodge_flash ?? true;
    form.quickAssist = combatSettings.ext_quick_assist ?? true;
    form.chainAttack = combatSettings.ext_chain ?? true;
    form.ultimateReady = combatSettings.ext_ultimate ?? true;
    form.useConcerto = combatSettings.use_concerto_switch ?? true;
    form.combatInterval = toNumber(combatSettings.interval, 0.15);
    form.pauseWhenBackground = config.app.Input.PauseWhenBackground;
    form.captureMethod = config.app.Capture.Method || "auto";
    form.inputMode = config.app.Input.Mode || "background";
    form.cursorMode = config.app.Input.CursorMode || "auto";
    form.ocrGpu = config.app.OCR?.GPU || "auto";
    form.gamepad = config.app.Input.Gamepad;
    form.excludedGames = [...(config.app.ExcludedGames ?? [])];
    form.hotkeyToggle = config.app.Hotkeys.Toggle || "Home";
    const featureKeys = config.app.Hotkeys.Features ?? {};
    form.hotkeyStorySkip = featureKeys[STORY_SKIP] ?? "F11";
    form.hotkeyAutoCombat = featureKeys[AUTO_COMBAT] ?? "F12";
    form.hotkeyGameOnly = config.app.Hotkeys.GameOnly ?? true;
    form.hotkeySound = config.app.Hotkeys.Sound ?? true;
    form.updateAutoCheck = config.app.Update?.AutoCheck ?? true;
    form.updateAutoDownload = config.app.Update?.AutoDownload ?? true;
  },
  { immediate: true },
);

function toggleExcluded(id: string) {
  if (form.excludedGames.includes(id)) {
    form.excludedGames = form.excludedGames.filter((item) => item !== id);
  } else {
    form.excludedGames = [...form.excludedGames, id];
  }
}

function save() {
  if (!props.config) return;
  const view: ConfigView = JSON.parse(JSON.stringify(props.config));

  // 通用配置写入 view.app.*，对所有游戏生效。
  view.app.Capture.Method = form.captureMethod;
  view.app.Input.Mode = form.inputMode;
  view.app.Input.CursorMode = form.cursorMode;
  view.app.Input.Gamepad = form.gamepad;
  view.app.Input.PauseWhenBackground = form.pauseWhenBackground;
  if (view.app.OCR) view.app.OCR.GPU = form.ocrGpu;
  view.app.ExcludedGames = [...form.excludedGames];
  view.app.Hotkeys.Toggle = form.hotkeyToggle.trim() || "Home";
  // 功能快捷键留空表示该功能不绑快捷键。
  view.app.Hotkeys.Features = {
    ...(view.app.Hotkeys.Features ?? {}),
    [STORY_SKIP]: form.hotkeyStorySkip.trim(),
    [AUTO_COMBAT]: form.hotkeyAutoCombat.trim(),
  };
  view.app.Hotkeys.GameOnly = form.hotkeyGameOnly;
  view.app.Hotkeys.Sound = form.hotkeySound;
  if (view.app.Update) {
    view.app.Update.AutoCheck = form.updateAutoCheck;
    view.app.Update.AutoDownload = form.updateAutoDownload;
  }

  // 当前游戏配置只写游戏确实拥有的功能，避免给没有该功能的游戏凭空造出配置项。
  const gameId = props.selectedGameId;
  if (gameId && featureIds.value.size > 0) {
    view.games = view.games ?? {};
    const game = (view.games[gameId] ??= { SchemaVersion: 1, Features: {} });
    const features = (game.Features ??= {});
    if (featureIds.value.has(STORY_SKIP)) {
      const feature = (features[STORY_SKIP] ??= { Enabled: true, Settings: {} });
      feature.Settings = {
        ...(feature.Settings ?? {}),
        mode: form.mode,
        dialog_option: form.dialogOption,
        dialog_click_interval: form.dialogClickInterval,
        option_click_interval: form.optionClickInterval,
        sleep_after_empty: form.sleepAfterEmpty,
      };
    }
    if (featureIds.value.has(AUTO_COMBAT)) {
      const feature = (features[AUTO_COMBAT] ??= { Enabled: true, Settings: {} });
      feature.Settings = {
        ...(feature.Settings ?? {}),
        use_liberation: form.useLiberation,
        use_echo: form.useEcho,
        use_dodge: form.useDodge,
        switch_healer: form.switchHealer,
        ext_dodge_flash: form.dodgeFlash,
        ext_quick_assist: form.quickAssist,
        ext_chain: form.chainAttack,
        ext_ultimate: form.ultimateReady,
        use_concerto_switch: form.useConcerto,
        interval: form.combatInterval,
      };
    }
  }
  emit("save", view);
}
</script>

<template>
  <section class="panel settings-panel">
    <div class="panel-head">
      <div>
        <span class="panel-kicker">偏好设置</span>
        <h2>调整跳过的行为</h2>
      </div>
      <span class="panel-meta">保存后立即生效</span>
    </div>

    <div class="settings-body">
      <!-- 通用配置：写入 view.app.*，对所有游戏生效 -->
      <section class="settings-group">
        <div class="group-head">
          <h3>通用配置</h3>
          <span class="group-meta">APP · 全部游戏共享</span>
        </div>

        <div class="settings-section">
          <div class="section-title"><span>01</span><h3>运行模式</h3></div>
          <div class="form-grid">
            <label class="field">
              <span>截图方式 / CAPTURE</span>
              <select v-model="form.captureMethod">
                <option value="auto">auto · 自动回退</option>
                <option value="printwindow">printwindow · 后台</option>
                <option value="bitblt">bitblt</option>
              </select>
            </label>
            <label class="field">
              <span>输入模式 / INPUT</span>
              <select v-model="form.inputMode">
                <option value="background">background · 后台，不影响操作</option>
                <option value="foreground">foreground · 前台 SendInput</option>
              </select>
            </label>
            <label class="field">
              <span>精确点击 / CURSOR</span>
              <select v-model="form.cursorMode">
                <option value="auto">auto · 不切前台：移光标 + 窗口消息（默认）</option>
                <option value="never">never · 纯消息，不移动光标（可能点不中）</option>
                <option value="always">always · 不切前台：移光标 + 窗口消息</option>
              </select>
            </label>
            <label class="field">
              <span>OCR 设备 / OCR BACKEND</span>
              <select v-model="form.ocrGpu">
                <option value="auto">auto · DirectML → CUDA → CPU</option>
                <option value="dml">dml · DirectML（Windows 通用）</option>
                <option value="cuda">cuda · NVIDIA CUDA</option>
                <option value="cpu">cpu · 仅 CPU</option>
              </select>
            </label>
            <label class="checkbox-field">
              <input v-model="form.pauseWhenBackground" type="checkbox" />
              <span>游戏窗口失去焦点时暂停（后台模式默认关闭）</span>
            </label>
            <label class="checkbox-field">
              <input v-model="form.gamepad" type="checkbox" />
              <span>启用虚拟 Xbox 手柄（需 ViGEmBus）</span>
            </label>
          </div>
        </div>

        <div class="settings-section">
          <div class="section-title"><span>02</span><h3>启动范围</h3></div>
          <p class="form-hint">默认自动启动全部游戏；取消勾选即可排除该游戏。</p>
          <div class="form-grid">
            <label v-for="game in games" :key="game.id" class="checkbox-field">
              <input
                type="checkbox"
                :checked="!form.excludedGames.includes(game.id)"
                @change="toggleExcluded(game.id)"
              />
              <span>{{ game.name }} · {{ game.id.toUpperCase() }}</span>
            </label>
            <p v-if="games.length === 0" class="empty-note">暂无可排除的游戏</p>
          </div>
        </div>

        <div class="settings-section">
          <div class="section-title"><span>03</span><h3>快捷键</h3></div>
          <div class="form-grid">
            <label class="field">
              <span>运行开关 / TOGGLE</span>
              <input v-model="form.hotkeyToggle" type="text" placeholder="Home" />
            </label>
            <label class="field">
              <span>剧情跳过开关 / STORY</span>
              <input v-model="form.hotkeyStorySkip" type="text" placeholder="F11（留空不绑定）" />
            </label>
            <label class="field">
              <span>自动战斗开关 / COMBAT</span>
              <input v-model="form.hotkeyAutoCombat" type="text" placeholder="F12（留空不绑定）" />
            </label>
            <label class="checkbox-field">
              <input v-model="form.hotkeyGameOnly" type="checkbox" />
              <span>只在游戏窗口前台时生效（不抢占其它应用的按键）</span>
            </label>
            <label class="checkbox-field">
              <input v-model="form.hotkeySound" type="checkbox" />
              <span>状态变化播放提示音</span>
            </label>
          </div>
          <p class="form-hint">
            运行开关按一次走一步：已停止 → 启动，运行中 → 暂停，已暂停 → 继续。托盘图标颜色随状态变化：灰=已停止、绿=运行中、琥珀=已暂停、红=异常。
          </p>
          <p class="form-hint">
            快捷键支持 F1–F24、字母数字、Home/End/Insert/Delete 等具名键，以及 Ctrl/Alt/Shift/Win 组合，例如 Home、F9、Ctrl+F11。由键盘钩子实现，F12 这类系统保留键同样可用。
          </p>
        </div>

        <div class="settings-section">
          <div class="section-title"><span>04</span><h3>更新</h3></div>
          <p class="form-hint">
            当前版本 {{ version || "dev" }}
            <span v-if="updateInfo?.available"> · 发现 {{ updateInfo.latest }}</span>
          </p>
          <div class="form-grid">
            <label class="checkbox-field">
              <input v-model="form.updateAutoCheck" type="checkbox" />
              <span>启动时自动检查更新</span>
            </label>
            <label class="checkbox-field">
              <input v-model="form.updateAutoDownload" type="checkbox" />
              <span>发现新版本后自动下载（安装仍由你确认）</span>
            </label>
          </div>
          <p v-if="updateDownloading" class="form-hint">正在下载更新… {{ updateProgress }}%</p>
          <p v-else-if="updateInfo?.downloaded" class="form-hint">新版本 {{ updateInfo.latest }} 已下载，点击“立即安装”完成更新。</p>
          <div class="form-actions">
            <button class="btn btn-ghost btn-small" :disabled="updateChecking || updateInstalling || updateDownloading" @click="emit('check-update')">
              {{ updateChecking ? "检查中…" : "检查更新" }}
            </button>
            <button
              v-if="updateInfo?.available && !updateInfo?.downloaded"
              class="btn btn-ghost btn-small"
              :disabled="updateDownloading"
              @click="emit('download-update')"
            >
              下载更新
            </button>
            <button v-if="updateInfo?.available" class="btn btn-primary btn-small" :disabled="updateInstalling || updateDownloading" @click="emit('install-update')">
              {{ updateInstalling ? "更新中…" : "立即安装" }}
            </button>
          </div>
          <p v-if="updateInfo?.notes" class="form-hint">{{ updateInfo.notes }}</p>
        </div>
      </section>

      <!-- 当前游戏配置：写入 view.games[gameId].Features.*，只显示该游戏拥有的功能 -->
      <section class="settings-group game-group">
        <div class="group-head">
          <h3>当前游戏配置 · {{ gameTitle }}</h3>
          <span class="group-meta">GAME · 仅当前选中游戏</span>
        </div>

        <div v-if="hasStorySkip" class="settings-section">
          <div class="section-title"><span>{{ storyIndex }}</span><h3>剧情推进</h3></div>
          <div class="form-grid">
            <label class="field">
              <span>推进模式 / MODE</span>
              <select v-model="form.mode">
                <option value="skip">skip · 跳过剧情</option>
                <option value="auto">auto · 等待自动播放</option>
                <option value="click">click · 只点对话</option>
              </select>
            </label>
            <label class="field">
              <span>选项策略 / DIALOG OPTION</span>
              <select v-model="form.dialogOption">
                <option value="first">first · 选择最上方</option>
                <option value="last">last · 选择最下方</option>
              </select>
            </label>
            <label class="field">
              <span>对话间隔 / DIALOG CLICK</span>
              <input v-model.number="form.dialogClickInterval" type="number" min="0.05" step="0.05" />
            </label>
            <label class="field">
              <span>选项间隔 / OPTION CLICK</span>
              <input v-model.number="form.optionClickInterval" type="number" min="0" step="0.05" />
            </label>
            <label class="field">
              <span>空画面等待 / IDLE WAIT</span>
              <input v-model.number="form.sleepAfterEmpty" type="number" min="0" step="0.05" />
            </label>
          </div>
        </div>

        <div v-if="hasAutoCombat" class="settings-section">
          <div class="section-title"><span>{{ combatIndex }}</span><h3>自动战斗</h3></div>
          <div class="form-grid">
            <label class="field field-switch">
              <span>使用爆发 / LIBERATION</span>
              <input v-model="form.useLiberation" type="checkbox" />
            </label>
            <label class="field field-switch">
              <span>使用装备技能 / ECHO</span>
              <input v-model="form.useEcho" type="checkbox" />
            </label>
            <label class="field field-switch">
              <span>使用闪避 / DODGE</span>
              <input v-model="form.useDodge" type="checkbox" />
            </label>
            <label class="field field-switch">
              <span>战斗前后切治疗 / HEALER</span>
              <input v-model="form.switchHealer" type="checkbox" />
            </label>
            <label class="field field-switch">
              <span>协奏满切人 / CONCERTO</span>
              <input v-model="form.useConcerto" type="checkbox" />
            </label>
            <label class="field">
              <span>出招间隔（秒） / INTERVAL</span>
              <input v-model.number="form.combatInterval" type="number" step="0.05" min="0.05" />
            </label>
          </div>
        </div>

        <p v-if="!hasStorySkip && !hasAutoCombat" class="empty-note">
          该游戏暂无可配置的功能，通用配置仍可保存。
        </p>
      </section>
    </div>

    <div class="form-actions">
      <button class="btn btn-primary" :disabled="!config || saving" @click="save">
        {{ saving ? "保存中…" : "保存配置" }}
      </button>
      <span v-if="!config" class="muted">配置加载中…</span>
      <span v-else class="form-hint">保存后立即生效；运行中的会话会自动重启</span>
    </div>
  </section>
</template>

<style scoped>
.settings-group {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px 16px 16px;
  border: 1px solid rgba(255, 255, 255, 0.72);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.32);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.85);
}
.game-group {
  border-color: rgba(10, 132, 255, 0.18);
  background: rgba(10, 132, 255, 0.05);
}
.group-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--line);
}
.group-head h3 {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.02em;
}
.group-meta {
  color: var(--muted);
  font-size: 9px;
  font-weight: 600;
  letter-spacing: 0.08em;
}
</style>
