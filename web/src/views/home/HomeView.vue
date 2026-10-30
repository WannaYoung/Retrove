<script setup lang="ts">
import { NIcon } from "naive-ui";
import {
  AlbumsOutline,
  AlertCircleOutline,
  CloudDownloadOutline,
  DiscOutline,
  GameControllerOutline,
  LayersOutline,
  ScanOutline,
} from "@vicons/ionicons5";
import { useAppStore } from "../app/composables/useApp";

const app = useAppStore();
</script>

<template>
  <section class="panel home">
    <div class="hero-line">
      <p>{{ app.t("app.tagline") }}</p>
    </div>
    <div class="stat-row">
      <div class="stat">
        <n-icon class="stat-icon" :component="LayersOutline" :size="18" />
        <span>{{ app.stats?.platforms ?? 0 }}</span>
        <label>{{ app.t("home.platforms") }}</label>
      </div>
      <div class="stat">
        <n-icon class="stat-icon" :component="GameControllerOutline" :size="18" />
        <span>{{ app.stats?.games ?? 0 }}</span>
        <label>{{ app.t("home.games") }}</label>
      </div>
      <div class="stat">
        <n-icon class="stat-icon" :component="DiscOutline" :size="18" />
        <span>{{ app.stats?.roms ?? 0 }}</span>
        <label>{{ app.t("home.roms") }}</label>
      </div>
      <div class="stat warn">
        <n-icon class="stat-icon" :component="AlertCircleOutline" :size="18" />
        <span>{{ app.stats?.missing ?? 0 }}</span>
        <label>{{ app.t("home.missing") }}</label>
      </div>
    </div>
    <div class="action-row">
      <h3>{{ app.t("home.actions") }}</h3>
      <div class="actions">
        <button class="pill" :disabled="app.busy" @click="app.runJob('scan')">
          <n-icon :component="ScanOutline" :size="18" />
          <span>{{ app.t("home.scanNow") }}</span>
        </button>
        <button class="pill" :disabled="app.busy" @click="app.runJob('import')">
          <n-icon :component="CloudDownloadOutline" :size="18" />
          <span>{{ app.t("home.importPegasus") }}</span>
        </button>
        <button class="pill ghost" @click="app.go('library')">
          <n-icon :component="AlbumsOutline" :size="18" />
          <span>{{ app.t("home.openLibrary") }}</span>
        </button>
      </div>
      <p v-if="app.busy" class="busy">{{ app.busyLabel }}</p>
    </div>
  </section>
</template>

<style scoped>
.hero-line p {
  margin: 0;
  opacity: 0.75;
}
</style>
