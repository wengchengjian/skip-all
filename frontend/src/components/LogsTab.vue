<script setup lang="ts">
import type { LogEntry } from "../api";
import LogList from "./LogList.vue";

defineProps<{
  logs: LogEntry[];
}>();
</script>

<template>
  <section class="panel logs-panel">
    <div class="panel-head">
      <div>
        <span class="panel-kicker">实时日志</span>
        <h2>运行日志</h2>
      </div>
      <div class="log-meta">
        <span class="badge">LIVE</span>
        <span class="panel-meta">{{ logs.length }} / 500</span>
      </div>
    </div>
    <div class="log-legend"><span class="legend-dot info"></span>INFO <span class="legend-dot warn"></span>WARN <span class="legend-dot error"></span>ERROR</div>
    <LogList :logs="logs" />
  </section>
</template>
