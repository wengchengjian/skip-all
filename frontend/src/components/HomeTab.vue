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
          :class="{ active: feature.id === selectedFeatureId }"
        >
          <input
            type="radio"
            name="feature"
            :checked="feature.id === selectedFeatureId"
            @change="emit('select-feature', feature.id)"
          />
          <span class="feature-radio"><i></i></span>
          <span class="feature-copy">
            <strong>{{ feature.name }}</strong>
            <small>{{ feature.description }}</small>
          </span>
          <span class="feature-code">{{ feature.id }}</span>
        </label>
        <p v-if="features.length === 0" class="empty-note">该游戏暂无可用协议</p>
      </div>

      <div class="mission-footer">
        <span class="micro-label">启动策略</span>
        <span class="mono-value">全部 · 自动检测</span>
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
