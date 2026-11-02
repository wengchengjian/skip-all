<script setup lang="ts">
import { computed, ref } from "vue";
import HomeTab from "./components/HomeTab.vue";
import LogsTab from "./components/LogsTab.vue";
import SettingsTab from "./components/SettingsTab.vue";
import { useSkipAll } from "./composables/useSkipAll";

type Tab = "home" | "settings" | "logs";

const tab = ref<Tab>("home");

const {
  games,
  config,
  status,
  logs,
  selectedGameId,
  selectedFeatureId,
  busy,
  modelReady,
  modelChecking,
  modelError,
  installing,
  modelProgress,
  modelMessage,
  doctor,
  doctorLoading,
  doctorError,
  version,
  updateInfo,
  updateChecking,
  updateInstalling,
  updateProgress,
  updateDownloading,
  notice,
  noticeType,
  stateLabel,
  selectGame,
  selectFeature,
  start,
  stop,
  pause,
  resume,
  saveConfig,
  toggleFeature,
  installModels,
  checkModels,
  runDoctor,
  checkUpdates,
  downloadUpdate,
  installUpdate,
} = useSkipAll();

const pageTitle = computed(() => ({ home: "任务总览", settings: "协议配置", logs: "运行日志" }[tab.value]));
const pageCode = computed(() => ({ home: "DASHBOARD", settings: "CONFIGURATION", logs: "EVENT STREAM" }[tab.value]));
const fpsLabel = computed(() =>
  status.value.running ? `${status.value.fps.toFixed(1)} / ${status.value.targetFps} FPS` : "— FPS",
);
// 节拍落后超过一拍（100ms）时把 chip 标黄，提示识别/截图拖慢了循环。
const tickLagging = computed(() => status.value.running && status.value.tickLagMs > 100);
const tickTitle = computed(() => {
  if (!status.value.running) return "";
  return `第 ${status.value.tickId} 拍，落后 ${status.value.tickLagMs.toFixed(0)}ms，已跳 ${status.value.tickDropped} 拍`;
});
</script>

<template>
  <div class="app-frame">
    <aside class="sidebar">
      <div class="brand-block">
        <div class="brand-sigil">S</div>
        <div class="brand-copy">
          <strong>SkipAll</strong>
          <span>游戏剧情跳过助手</span>
        </div>
      </div>

      <nav class="nav-block" aria-label="主导航">
        <button class="nav-item" :class="{ active: tab === 'home' }" @click="tab = 'home'">
          <span class="nav-index">01</span>
          <span class="nav-label">首页</span>
          <span class="nav-line"></span>
        </button>
        <button class="nav-item" :class="{ active: tab === 'settings' }" @click="tab = 'settings'">
          <span class="nav-index">02</span>
          <span class="nav-label">设置</span>
          <span class="nav-line"></span>
        </button>
        <button class="nav-item" :class="{ active: tab === 'logs' }" @click="tab = 'logs'">
          <span class="nav-index">03</span>
          <span class="nav-label">日志</span>
          <span class="nav-line"></span>
        </button>
      </nav>

      <div class="sidebar-foot">
        <span>BUILD {{ version || "dev" }}</span>
        <span>WIN / X64</span>
      </div>
    </aside>

    <section class="workspace">
      <header class="topbar">
        <div class="page-heading">
          <span class="page-kicker">SKIPALL · {{ pageCode }}</span>
          <h1>{{ pageTitle }}</h1>
        </div>
        <div class="status-cluster">
          <span class="fps-chip" :class="{ live: status.running, lagging: tickLagging }" :title="tickTitle">{{ fpsLabel }}</span>
          <span v-if="status.ocrBackend" class="ocr-chip">OCR {{ status.ocrBackend }}</span>
          <div class="status-chip" :class="status.state">
            <span class="status-pulse"></span>
            <span>{{ stateLabel }}</span>
            <span class="status-sep">/</span>
            <span class="status-feature">{{ status.featureName || "STANDBY" }}</span>
          </div>
        </div>
      </header>

      <main class="content">
        <HomeTab
          v-if="tab === 'home'"
          :games="games"
          :config="config"
          :selected-game-id="selectedGameId"
          :selected-feature-id="selectedFeatureId"
          :status="status"
          :logs="logs"
          :busy="busy"
          :model-ready="modelReady"
          :model-checking="modelChecking"
          :model-error="modelError"
          :installing="installing"
          :model-progress="modelProgress"
          :model-message="modelMessage"
          :doctor="doctor"
          :doctor-loading="doctorLoading"
          :doctor-error="doctorError"
          @select-game="selectGame"
          @select-feature="selectFeature"
          @toggle-feature="(featureId, enabled) => toggleFeature(selectedGameId, featureId, enabled)"
          @start="start"
          @stop="stop"
          @pause="pause"
          @resume="resume"
          @install-models="installModels"
          @check-models="checkModels"
          @doctor="runDoctor"
        />
        <SettingsTab
          v-else-if="tab === 'settings'"
          :config="config"
          :games="games"
          :selected-game-id="selectedGameId"
          :saving="busy"
          :version="version"
          :update-info="updateInfo"
          :update-checking="updateChecking"
          :update-installing="updateInstalling"
          :update-downloading="updateDownloading"
          :update-progress="updateProgress"
          @save="saveConfig"
          @check-update="checkUpdates"
          @download-update="downloadUpdate"
          @install-update="installUpdate"
        />
        <LogsTab v-else :logs="logs" />
      </main>
    </section>

    <transition name="toast">
      <div v-if="notice" class="toast" :class="noticeType">{{ notice }}</div>
    </transition>
  </div>
</template>
