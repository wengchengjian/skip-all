import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { Events } from "@wailsio/runtime";
import { Service } from "../api";
import type { ConfigView, DoctorReport, GameView, LogEntry, StatusView } from "../api";

function errorText(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export function useSkipAll() {
  const games = ref<GameView[]>([]);
  const config = ref<ConfigView | null>(null);
  const status = ref<StatusView>({
    state: "idle",
    running: false,
    paused: false,
    gameId: "",
    featureId: "",
    featureName: "",
    message: "",
    error: "",
    startedAt: "",
    fps: 0,
    ocrBackend: "",
  });
  const logs = ref<LogEntry[]>([]);
  const selectedGameId = ref("zzz");
  const selectedFeatureId = ref("story_skip");
  const busy = ref(false);

  const modelReady = ref(false);
  const modelChecking = ref(true);
  const modelError = ref("");
  const installing = ref(false);
  const modelProgress = ref(0);
  const modelMessage = ref("");

  const doctor = ref<DoctorReport | null>(null);
  const doctorLoading = ref(false);
  const doctorError = ref("");

  const notice = ref("");
  const noticeType = ref<"ok" | "error">("ok");
  let noticeTimer: number | undefined;

  const selectedGame = computed<GameView | null>(
    () => games.value.find((game) => game.id === selectedGameId.value) ?? null,
  );
  const selectedFeature = computed(() => {
    const features = selectedGame.value?.features ?? [];
    return features.find((feature) => feature.id === selectedFeatureId.value) ?? null;
  });
  const active = computed(() => status.value.running || status.value.paused);
  const stateLabel = computed(() => {
    const labels: Record<string, string> = {
      idle: "空闲",
      running: "运行中",
      paused: "已暂停",
      stopped: "已停止",
      error: "出错",
    };
    return labels[status.value.state] ?? status.value.state;
  });

  function showNotice(text: string, type: "ok" | "error" = "ok") {
    notice.value = text;
    noticeType.value = type;
    window.clearTimeout(noticeTimer);
    noticeTimer = window.setTimeout(() => {
      notice.value = "";
    }, 4000);
  }

  function addLog(entry: LogEntry) {
    logs.value = [...logs.value, entry].slice(-500);
  }

  async function loadGames() {
    try {
      games.value = (await Service.ListGames()) ?? [];
      if (!games.value.some((game) => game.id === selectedGameId.value)) {
        const first = games.value[0];
        if (first) {
          selectedGameId.value = first.id;
          selectedFeatureId.value = first.features?.[0]?.id ?? "";
        }
      } else if (!selectedFeature.value && selectedGame.value?.features?.[0]) {
        selectedFeatureId.value = selectedGame.value.features[0].id;
      }
    } catch (err) {
      showNotice(`加载游戏列表失败：${errorText(err)}`, "error");
    }
  }

  async function loadConfig() {
    try {
      config.value = await Service.GetConfig();
    } catch (err) {
      showNotice(`加载配置失败：${errorText(err)}`, "error");
    }
  }

  async function refreshStatus() {
    try {
      status.value = await Service.GetStatus();
    } catch (err) {
      showNotice(`读取状态失败：${errorText(err)}`, "error");
    }
  }

  async function loadLogs() {
    try {
      logs.value = (await Service.GetLogs()) ?? [];
    } catch (err) {
      showNotice(`加载日志失败：${errorText(err)}`, "error");
    }
  }

  async function checkModels() {
    modelChecking.value = true;
    try {
      await Service.EnsureModels();
      modelReady.value = true;
      modelError.value = "";
    } catch (err) {
      modelReady.value = false;
      modelError.value = errorText(err);
    } finally {
      modelChecking.value = false;
    }
  }

  async function installModels() {
    if (installing.value) return;
    installing.value = true;
    modelProgress.value = 0;
    modelMessage.value = "准备安装…";
    try {
      await Service.InstallModels();
      modelReady.value = true;
      modelError.value = "";
      showNotice("OCR 模型安装完成");
    } catch (err) {
      modelError.value = errorText(err);
      showNotice(`OCR 模型安装失败：${modelError.value}`, "error");
    } finally {
      installing.value = false;
      modelMessage.value = "";
    }
  }

  function selectGame(id: string) {
    selectedGameId.value = id;
    const game = games.value.find((item) => item.id === id);
    const first = game?.features?.[0];
    if (first) selectedFeatureId.value = first.id;
  }

  function selectFeature(id: string) {
    selectedFeatureId.value = id;
  }

  async function start() {
    if (!selectedGameId.value || !selectedFeatureId.value) {
      showNotice("请先选择游戏和功能", "error");
      return;
    }
    busy.value = true;
    try {
      const result = await Service.StartAll();
      if (result.ok) {
        showNotice(`已启动：${selectedFeature.value?.name ?? selectedFeatureId.value}`);
      } else {
        showNotice(result.error || "启动失败", "error");
      }
      await refreshStatus();
    } catch (err) {
      showNotice(`启动失败：${errorText(err)}`, "error");
    } finally {
      busy.value = false;
    }
  }

  async function stop() {
    busy.value = true;
    try {
      await Service.StopAll();
      await refreshStatus();
    } catch (err) {
      showNotice(`停止失败：${errorText(err)}`, "error");
    } finally {
      busy.value = false;
    }
  }

  async function pause() {
    busy.value = true;
    try {
      await Service.PauseAll();
      await refreshStatus();
    } catch (err) {
      showNotice(`暂停失败：${errorText(err)}`, "error");
    } finally {
      busy.value = false;
    }
  }

  async function resume() {
    busy.value = true;
    try {
      await Service.ResumeAll();
      await refreshStatus();
    } catch (err) {
      showNotice(`恢复失败：${errorText(err)}`, "error");
    } finally {
      busy.value = false;
    }
  }

  async function saveConfig(view: ConfigView) {
    busy.value = true;
    try {
      await Service.SaveConfig(view);
      config.value = view;
      showNotice("设置已保存，运行中的会话已自动重启");
    } catch (err) {
      showNotice(`保存配置失败：${errorText(err)}`, "error");
    } finally {
      busy.value = false;
    }
  }

  async function runDoctor() {
    doctorLoading.value = true;
    doctorError.value = "";
    try {
      doctor.value = await Service.Doctor(selectedGameId.value);
    } catch (err) {
      doctor.value = null;
      doctorError.value = errorText(err);
    } finally {
      doctorLoading.value = false;
    }
  }

  const offs: Array<() => void> = [];
  let statusTimer: number | undefined;

  onMounted(async () => {
    offs.push(
      Events.On("skipall:status", (event) => {
        status.value = event.data as StatusView;
      }),
      Events.On("skipall:log", (event) => {
        addLog(event.data as LogEntry);
      }),
      Events.On("skipall:model", (event) => {
        const data = event.data as { percent?: number; message?: string };
        installing.value = true;
        modelProgress.value = Math.min(100, Math.max(0, data.percent ?? 0));
        modelMessage.value = data.message ?? "";
      }),
    );
    statusTimer = window.setInterval(refreshStatus, 1000);
    await Promise.all([loadGames(), loadConfig(), refreshStatus(), loadLogs(), checkModels()]);
  });

  onBeforeUnmount(() => {
    offs.forEach((off) => off());
    window.clearInterval(statusTimer);
    window.clearTimeout(noticeTimer);
  });

  return {
    games,
    config,
    status,
    logs,
    selectedGameId,
    selectedFeatureId,
    selectedGame,
    selectedFeature,
    active,
    stateLabel,
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
    notice,
    noticeType,
    selectGame,
    selectFeature,
    start,
    stop,
    pause,
    resume,
    saveConfig,
    installModels,
    checkModels,
    runDoctor,
  };
}
