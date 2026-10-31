<script setup lang="ts">
import { nextTick, ref, watch } from "vue";
import type { LogEntry } from "../api";

const props = defineProps<{
  logs: LogEntry[];
  compact?: boolean;
}>();

const container = ref<HTMLElement | null>(null);

function formatTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "--:--:--";
  return date.toLocaleTimeString("zh-CN", { hour12: false });
}

watch(
  () => props.logs.length,
  async () => {
    await nextTick();
    const el = container.value;
    if (el) el.scrollTop = el.scrollHeight;
  },
);
</script>

<template>
  <div ref="container" class="log-list" :class="{ compact }">
    <p v-if="logs.length === 0" class="muted">暂无日志</p>
    <div v-for="(entry, index) in logs" :key="index" class="log-line">
      <span class="log-time">{{ formatTime(entry.time) }}</span>
      <span class="log-level" :class="entry.level">{{ entry.level }}</span>
      <span class="log-msg">{{ entry.message }}</span>
    </div>
  </div>
</template>

<style scoped>
.log-list.compact {
  max-height: 170px;
}
</style>
