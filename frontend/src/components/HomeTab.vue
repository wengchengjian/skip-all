<script setup lang="ts">
import { computed } from "vue";
import type { ConfigView, DoctorReport, GameView, LogEntry, StatusView } from "../api";
import LogList from "./LogList.vue";

const props = defineProps<{
  games: GameView[];
  config: ConfigView | null;
  selectedGameId: string;
  selectedFeatureId: string;
  status: StatusView;
  logs: LogEntry[];
  busy: boolean;
  modelReady: boolean;
  modelChecking: boolean;
  modelError: string;
  installing: boolean;
  modelProgress: number;
  modelMessage: string;
  doctor: DoctorReport | null;
  doctorLoading: boolean;
  doctorError: string;
}>();

const emit = defineEmits<{
  (e: "select-game", id: string): void;
  (e: "select-feature", id: string): void;
  (e: "toggle-feature", featureId: string, enabled: boolean): void;
  (e: "start"): void;
  (e: "stop"): void;
  (e: "pause"): void;
  (e: "resume"): void;
  (e: "install-models"): void;
  (e: "check-models"): void;
  (e: "doctor"): void;
}>();

const selectedGame = computed(() => props.games.find((game) => game.id === props.selectedGameId) ?? null);
const excludedGames = computed(() => new Set(props.config?.app.ExcludedGames ?? []));
function gameState(game: GameView): string {
  if (excludedGames.value.has(game.id)) return "已排除";
  if (props.status.gameId.split(",").includes(game.id)) return "运行中";
  return "已启用";
}
const features = computed(() => selectedGame.value?.features ?? []);
const selectedFeature = computed(() => features.value.find((feature) => feature.id === props.selectedFeatureId) ?? null);
// 勾选状态读配置：ConfigView 的 JSON 字段是 Go 字段名（Features/Enabled），缺省视为启用。
function featureEnabled(featureId: string): boolean {
  return props.config?.games?.[props.selectedGameId]?.Features?.[featureId]?.Enabled ?? true;
}
// 功能运行状态来自 status.features（每个游戏每个功能一条），找不到即未运行。
function featureStatus(featureId: string) {
  return props.status.features?.find(
    (item) => item.gameId === props.selectedGameId && item.featureId === featureId,
  ) ?? null;
}
const FEATURE_STATE_LABELS: Record<string, string> = {
  running: "运行中",
  paused: "已暂停",
  stopped: "已停止",
  error: "异常",
  idle: "空闲",
};
function featureState(featureId: string): string {
  return FEATURE_STATE_LABELS[featureStatus(featureId)?.state ?? ""] ?? "未运行";
}
const recentLogs = computed(() => props.logs.slice(-7));
const sessionActive = computed(() => props.status.running || props.status.paused);
const statusLabel = computed(() => {
  const labels: Record<string, string> = { idle: "待命", running: "运行中", paused: "已暂停", stopped: "已停止", error: "异常" };
  return labels[props.status.state] ?? props.status.state;
});
const sessionTime = computed(() => {
  if (!props.status.running && !props.status.paused) return "--:--";
  if (!props.status.startedAt) return "--:--";
  const date = new Date(props.status.startedAt);
  if (Number.isNaN(date.getTime())) return "--:--";
  return date.toLocaleTimeString("zh-CN", { hour12: false, hour: "2-digit", minute: "2-digit" });
});
</script>

<template>
  <div class="home-grid">
    <section class="panel mission-panel">
      <div class="panel-head">
        <div>
          <span class="panel-kicker">自动跳过范围</span>
          <h2>全部游戏</h2>
        </div>
        <span class="panel-meta">{{ games.length }} 个目标</span>
      </div>

      <div class="game-strip">
        <button
          v-for="(game, index) in games"
          :key="game.id"
          class="game-tile"
          :class="{ active: game.id === selectedGameId }"
          @click="emit('select-game', game.id)"
        >
          <span class="game-index">0{{ index + 1 }}</span>
          <strong>{{ game.name }}</strong>
          <small>{{ game.id.toUpperCase() }}</small>
          <span class="game-state">{{ gameState(game) }}</span>
        </button>
        <p v-if="games.length === 0" class="empty-note">未发现内置游戏</p>
      </div>

      <div class="feature-list">
        <label
          v-for="feature in features"
          :key="feature.id"
          class="feature-row"
          :class="{ active: featureEnabled(feature.id) }"
        >
          <input
            type="checkbox"
            :checked="featureEnabled(feature.id)"
            @change="emit('toggle-feature', feature.id, !featureEnabled(feature.id))"
          />
          <span class="feature-radio"><i></i></span>
          <span class="feature-copy">
            <strong>{{ feature.name }}</strong>
            <small>{{ feature.description }}</small>
          </span>
          <span class="feature-tail">
            <span class="feature-code">{{ feature.id }}</span>
            <span class="feature-state" :data-state="featureStatus(feature.id)?.state ?? 'none'">
              {{ featureState(feature.id) }}
            </span>
          </span>
        </label>
        <p v-if="features.length === 0" class="empty-note">该游戏暂无可用协议</p>
      </div>

      <div class="mission-footer">
        <span class="micro-label">启动策略</span>
        <span class="mono-value">全部启用功能 · 自动检测</span>
      </div>
    </section>

    <section class="panel command-panel">
      <div class="panel-head">
        <div>
          <span class="panel-kicker">运行控制</span>
          <h2>当前状态</h2>
        </div>
        <span class="panel-meta status-code" :class="status.state">{{ status.state.toUpperCase() }}</span>
      </div>

      <div class="status-readout" :class="status.state">
        <div class="status-orbit">
          <div class="status-core">
            <span class="status-label">{{ statusLabel }}</span>
            <strong>{{ status.featureName || selectedFeature?.name || "未选择协议" }}</strong>
          </div>
        </div>
        <p class="status-message">{{ status.message || "选择协议后启动，程序会自动识别剧情并推进。" }}</p>
        <p v-if="status.error" class="error-text">{{ status.error }}</p>
      </div>

      <div class="command-actions">
        <button class="btn btn-primary" :disabled="busy" @click="emit('start')">开始全部</button>
        <button class="btn btn-ghost" :disabled="busy || !status.running" @click="emit('pause')">暂停</button>
        <button v-if="status.paused" class="btn btn-ghost" :disabled="busy" @click="emit('resume')">继续</button>
        <button class="btn btn-danger" :disabled="busy || !sessionActive" @click="emit('stop')">停止</button>
      </div>

      <div class="telemetry">
        <div><span>GAME</span><strong>{{ status.gameId ? status.gameId.toUpperCase() : "ALL" }}</strong></div>
        <div><span>FEATURE</span><strong>{{ status.featureId || selectedFeature?.id || "—" }}</strong></div>
        <div><span>STARTED</span><strong>{{ sessionTime }}</strong></div>
      </div>
    </section>

    <section class="panel model-panel">
      <div class="panel-head compact-head">
        <div>
          <span class="panel-kicker">视觉识别</span>
          <h2>OCR 模型</h2>
        </div>
        <span v-if="modelChecking" class="badge">CHECKING</span>
        <span v-else-if="modelReady" class="badge ok">READY</span>
        <span v-else class="badge bad">MISSING</span>
      </div>
      <p v-if="!modelChecking && !modelReady" class="status-message">模型缺失或文件不完整，启动前请先安装。</p>
      <p v-if="modelError" class="error-text">{{ modelError }}</p>
      <div v-if="installing" class="progress"><span :style="{ width: modelProgress + '%' }"></span></div>
      <p v-if="installing" class="status-message">{{ modelMessage || "安装中…" }} {{ Math.round(modelProgress) }}%</p>
      <div class="model-actions">
        <button v-if="!modelReady" class="btn btn-primary btn-small" :disabled="installing" @click="emit('install-models')">安装模型</button>
        <button class="btn btn-ghost btn-small" :disabled="modelChecking || installing" @click="emit('check-models')">重新检查</button>
        <button class="btn btn-ghost btn-small" :disabled="doctorLoading" @click="emit('doctor')">{{ doctorLoading ? "检测中…" : "环境检测" }}</button>
      </div>
      <div v-if="doctor" class="doctor compact">
        <span>{{ doctor.windowTitle || "未找到窗口" }}</span>
        <span>{{ doctor.captureWidth }}×{{ doctor.captureHeight }} / {{ doctor.scale }}x</span>
        <span>OCR {{ doctor.ocrReady ? "OK" : "FAIL" }}</span>
      </div>
      <p v-if="doctorError" class="error-text">{{ doctorError }}</p>
    </section>

    <section class="panel log-panel">
      <div class="panel-head compact-head">
        <div>
          <span class="panel-kicker">实时反馈</span>
          <h2>最近日志</h2>
        </div>
        <span class="panel-meta">{{ logs.length }} 条</span>
      </div>
      <LogList :logs="recentLogs" compact />
    </section>
  </div>
</template>

<style scoped>
/* 单选圆点改为方角勾选框，匹配多选语义。 */
.feature-row .feature-radio { border-radius: 5px; }
.feature-row .feature-radio i { width: 8px; height: 8px; border-radius: 2px; }
/* 功能行右侧：功能 ID + 运行状态徽标，沿用全局 .badge 的胶囊风格。 */
.feature-tail {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 5px;
  flex: 0 0 auto;
}
.feature-state {
  padding: 1px 7px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.55);
  color: var(--muted);
  font-size: 9px;
  font-weight: 600;
  letter-spacing: 0.04em;
  white-space: nowrap;
}
.feature-state[data-state="running"] { border-color: rgba(52, 199, 89, 0.18); background: rgba(52, 199, 89, 0.1); color: #1f9d45; }
.feature-state[data-state="paused"] { border-color: rgba(255, 159, 10, 0.2); background: rgba(255, 159, 10, 0.1); color: #c07600; }
.feature-state[data-state="stopped"] { border-color: rgba(15, 23, 42, 0.1); color: var(--muted); }
.feature-state[data-state="error"] { border-color: rgba(255, 59, 48, 0.16); background: rgba(255, 59, 48, 0.08); color: var(--red); }
.feature-state[data-state="idle"] { border-color: rgba(10, 132, 255, 0.16); background: rgba(10, 132, 255, 0.08); color: var(--blue-strong); }
</style>
