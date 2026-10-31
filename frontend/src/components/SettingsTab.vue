<script setup lang="ts">
import { reactive, watch } from "vue";
import type { ConfigView } from "../api";

const props = defineProps<{
  config: ConfigView | null;
  saving: boolean;
}>();

const emit = defineEmits<{
  (e: "save", view: ConfigView): void;
}>();

const form = reactive({
  mode: "skip",
  dialogOption: "last",
  dialogClickInterval: 0.5,
  optionClickInterval: 0.1,
  sleepAfterEmpty: 0.5,
  pauseWhenBackground: true,
  captureMethod: "auto",
  inputMode: "background",
  cursorMode: "never",
  gamepad: false,
  hotkeyToggle: "F9",
  hotkeyPanic: "F10",
});

function toNumber(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

watch(
  () => props.config,
  (config) => {
    if (!config) return;
    const feature = config.zzz.Features?.["story_skip"];
    const settings = feature?.Settings ?? {};
    form.mode = String(settings.mode ?? "skip");
    form.dialogOption = String(settings.dialog_option ?? "last");
    form.dialogClickInterval = toNumber(settings.dialog_click_interval, 0.5);
    form.optionClickInterval = toNumber(settings.option_click_interval, 0.1);
    form.sleepAfterEmpty = toNumber(settings.sleep_after_empty, 0.5);
    form.pauseWhenBackground = config.app.Input.PauseWhenBackground;
    form.captureMethod = config.app.Capture.Method || "auto";
    form.inputMode = config.app.Input.Mode || "background";
    form.cursorMode = config.app.Input.CursorMode || "never";
    form.gamepad = config.app.Input.Gamepad;
    form.hotkeyToggle = config.app.Hotkeys.Toggle || "F9";
    form.hotkeyPanic = config.app.Hotkeys.Panic || "F10";
  },
  { immediate: true },
);

function save() {
  if (!props.config) return;
  const view: ConfigView = JSON.parse(JSON.stringify(props.config));
  view.app.Capture.Method = form.captureMethod;
  view.app.Input.Mode = form.inputMode;
  view.app.Input.CursorMode = form.cursorMode;
  view.app.Input.Gamepad = form.gamepad;
  view.app.Input.PauseWhenBackground = form.pauseWhenBackground;
  view.app.Hotkeys.Toggle = form.hotkeyToggle.trim() || "F9";
  view.app.Hotkeys.Panic = form.hotkeyPanic.trim() || "F10";

  const features = view.zzz.Features ?? {};
  view.zzz.Features = features;
  let feature = features["story_skip"];
  if (!feature) {
    feature = { Enabled: true, Settings: {} };
    features["story_skip"] = feature;
  }
  feature.Settings = {
    ...(feature.Settings ?? {}),
    mode: form.mode,
    dialog_option: form.dialogOption,
    dialog_click_interval: form.dialogClickInterval,
    option_click_interval: form.optionClickInterval,
    sleep_after_empty: form.sleepAfterEmpty,
  };
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
      <span class="panel-meta">下次启动生效</span>
    </div>

    <div class="settings-body">
      <section class="settings-section">
        <div class="section-title"><span>01</span><h3>剧情推进</h3></div>
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
          <label class="field">
            <span>精确点击 / CURSOR</span>
            <select v-model="form.cursorMode">
              <option value="never">never · 不移动前台鼠标（默认）</option>
              <option value="auto">auto · 后台空闲时短暂移光标并归位</option>
              <option value="always">always · 总是移光标</option>
            </select>
          </label>
        </div>
      </section>

      <section class="settings-section">
        <div class="section-title"><span>02</span><h3>运行时</h3></div>
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
            <span>切换快捷键 / TOGGLE</span>
            <input v-model="form.hotkeyToggle" type="text" placeholder="F9" />
          </label>
          <label class="field">
            <span>急停快捷键 / PANIC</span>
            <input v-model="form.hotkeyPanic" type="text" placeholder="F10" />
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
      </section>
    </div>

    <div class="form-actions">
      <button class="btn btn-primary" :disabled="!config || saving" @click="save">
        {{ saving ? "保存中…" : "保存配置" }}
      </button>
      <span v-if="!config" class="muted">配置加载中…</span>
      <span v-else class="form-hint">修改将在下一次启动会话时生效</span>
    </div>
  </section>
</template>
