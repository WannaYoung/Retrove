<script setup lang="ts">
import { computed } from "vue";
import { NIcon, NTag } from "naive-ui";
import {
  AlertCircleOutline,
  CloudDownloadOutline,
  GameControllerOutline,
  HelpCircleOutline,
  ImageOutline,
  LanguageOutline,
  LayersOutline,
  ScanOutline,
} from "@vicons/ionicons5";
import { mediaURL, type Job } from "../../api/client";
import { tileLetter, useAppStore } from "../app/composables/useApp";

const app = useAppStore();

const games = computed(() => app.stats?.games ?? 0);
const unnamed = computed(() => app.stats?.unidentified ?? 0);
const pendingEn = computed(() => app.stats?.pending_en ?? 0);
const missingCovers = computed(() => app.stats?.missing_covers ?? 0);
const missingDesc = computed(() => app.stats?.missing_description ?? 0);
const missingFiles = computed(() => app.stats?.missing ?? 0);
const named = computed(() => Math.max(0, games.value - unnamed.value));
const withEn = computed(() => Math.max(0, named.value - pendingEn.value));
const withCover = computed(() => Math.max(0, games.value - missingCovers.value));
const withDesc = computed(() => Math.max(0, games.value - missingDesc.value));

function pct(part: number) {
  if (games.value <= 0) return 0;
  return Math.min(100, Math.round((Math.max(0, part) / games.value) * 100));
}

const completeness = computed(() => [
  { key: "zh", labelKey: "home.hasTitleZh", value: named.value, pct: pct(named.value) },
  { key: "en", labelKey: "home.hasTitleEn", value: withEn.value, pct: pct(withEn.value) },
  { key: "cover", labelKey: "home.hasCover", value: withCover.value, pct: pct(withCover.value) },
  { key: "desc", labelKey: "home.hasDescription", value: withDesc.value, pct: pct(withDesc.value) },
]);

const platformMax = computed(() =>
  Math.max(1, ...app.romPlatforms.map((p) => p.rom_count ?? 0), 1),
);

const recentJobs = computed(() => app.jobs.slice(0, 5));

function jobTypeLabel(type: string) {
  const key = `jobs.types.${type}`;
  const label = app.t(key);
  return label === key ? type : label;
}

function jobStatusLabel(status: string) {
  const key = `jobs.status.${status}`;
  const label = app.t(key);
  return label === key ? status : label;
}

function jobStatusType(status: string): "success" | "error" | "info" | "default" {
  if (status === "succeeded") return "success";
  if (status === "failed") return "error";
  if (status === "running") return "info";
  return "default";
}

function jobKey(job: Job) {
  return job.id;
}
</script>

<template>
  <section class="panel home">
    <div class="hero-line">
      <h2>{{ app.t("home.title") }}</h2>
      <div class="hero-actions">
        <button class="pill" :disabled="app.busy" @click="app.runJob('scan')">
          <n-icon :component="ScanOutline" :size="18" />
          <span>{{ app.t("home.scanNow") }}</span>
        </button>
        <button class="pill" :disabled="app.busy" @click="app.runJob('import')">
          <n-icon :component="CloudDownloadOutline" :size="18" />
          <span>{{ app.t("home.importPegasus") }}</span>
        </button>
      </div>
    </div>
    <p v-if="app.busy" class="busy hero-busy">{{ app.busyLabel }}</p>

    <div class="stat-row">
      <button class="stat" @click="app.openLibrary()">
        <n-icon class="stat-icon" :component="GameControllerOutline" :size="18" />
        <span>{{ games }}</span>
        <label>{{ app.t("home.games") }}</label>
      </button>
      <button class="stat" @click="app.openLibrary()">
        <n-icon class="stat-icon" :component="LayersOutline" :size="18" />
        <span>{{ app.stats?.platforms_with_roms ?? 0 }}</span>
        <label>{{ app.t("home.platforms") }}</label>
      </button>
      <button class="stat" :class="{ warn: pendingEn > 0 }" @click="app.go('settings', 'translate')">
        <n-icon class="stat-icon" :component="LanguageOutline" :size="18" />
        <span>{{ pendingEn }}</span>
        <label>{{ app.t("home.pendingEn") }}</label>
      </button>
      <button class="stat" :class="{ warn: missingCovers > 0 }" @click="app.go('scrape')">
        <n-icon class="stat-icon" :component="ImageOutline" :size="18" />
        <span>{{ missingCovers }}</span>
        <label>{{ app.t("home.missingCovers") }}</label>
      </button>
    </div>

    <div v-if="unnamed > 0 || missingFiles > 0" class="issue-row">
      <button v-if="unnamed > 0" class="issue" @click="app.openLibrary()">
        <n-icon :component="HelpCircleOutline" :size="16" />
        {{ unnamed }} {{ app.t("home.unidentified") }}
      </button>
      <button v-if="missingFiles > 0" class="issue" @click="app.openLibrary()">
        <n-icon :component="AlertCircleOutline" :size="16" />
        {{ missingFiles }} {{ app.t("home.missing") }}
      </button>
    </div>

    <div class="home-split">
      <div class="home-block">
        <h3>{{ app.t("home.completeness") }}</h3>
        <ul class="meter-list">
          <li v-for="item in completeness" :key="item.key">
            <div class="meter-head">
              <span>{{ app.t(item.labelKey) }}</span>
              <strong>{{ item.pct }}%</strong>
            </div>
            <div class="meter" :class="{ warn: item.pct < 50 }">
              <i :style="{ width: `${item.pct}%` }" />
            </div>
            <small>{{ item.value }} / {{ games }}</small>
          </li>
        </ul>
      </div>
      <div class="home-block">
        <h3>{{ app.t("home.platformList") }}</h3>
        <p v-if="!app.romPlatforms.length" class="empty">{{ app.t("home.noPlatforms") }}</p>
        <div v-else class="platform-list">
          <button
            v-for="p in app.romPlatforms"
            :key="p.id"
            class="platform-row"
            @click="app.openLibrary(p.id)"
          >
            <div class="meter-head">
              <strong>{{ p.name }}</strong>
              <span>{{ p.rom_count ?? 0 }}</span>
            </div>
            <div class="meter">
              <i :style="{ width: `${Math.round(((p.rom_count ?? 0) / platformMax) * 100)}%` }" />
            </div>
          </button>
        </div>
      </div>
    </div>

    <div class="home-block">
      <div class="block-head">
        <h3>{{ app.t("home.recentGames") }}</h3>
        <button v-if="app.recentGames.length" class="text-link" @click="app.openLibrary()">
          {{ app.t("home.openLibrary") }}
        </button>
      </div>
      <p v-if="!app.recentGames.length" class="empty">{{ app.t("home.recentGamesEmpty") }}</p>
      <div v-else class="cover-strip">
        <button v-for="g in app.recentGames" :key="g.id" class="cover" @click="app.openGame(g)">
          <div class="art" :data-tone="g.platform_id.length % 5">
            <img v-if="g.cover_path" :src="mediaURL(g.cover_path)" alt="" loading="lazy" />
            <span v-else>{{ tileLetter(g.title_zh || g.filename) }}</span>
          </div>
          <div class="meta">
            <strong>{{ g.title_zh || g.filename }}</strong>
            <small>{{ g.platform_name }}</small>
          </div>
        </button>
      </div>
    </div>

    <div class="home-split">
      <div class="home-block">
        <div class="block-head">
          <h3>{{ app.t("home.sources") }}</h3>
          <button class="text-link" @click="app.go('settings', 'providers')">
            {{ app.t("home.configureSources") }}
          </button>
        </div>
        <div class="source-chips">
          <n-tag size="small" round :type="app.scrapeStatus?.igdb_configured ? 'success' : 'default'">
            {{ app.t("scrape.sourceIGDB") }}
          </n-tag>
          <n-tag size="small" round :type="app.scrapeStatus?.screenscraper_configured ? 'success' : 'default'">
            {{ app.t("scrape.sourceSS") }}
          </n-tag>
          <n-tag size="small" round :type="app.scrapeStatus?.steamgriddb_configured ? 'success' : 'default'">
            {{ app.t("scrape.sourceSGDB") }}
          </n-tag>
          <n-tag size="small" round :type="app.llmConfigured ? 'success' : 'default'">
            {{ app.t("settings.translate") }}
          </n-tag>
        </div>
      </div>
      <div class="home-block">
        <div class="block-head">
          <h3>{{ app.t("jobs.recent") }}</h3>
          <button class="text-link" @click="app.go('settings', 'tasks')">{{ app.t("jobs.open") }}</button>
        </div>
        <p v-if="!recentJobs.length" class="empty">{{ app.t("jobs.empty") }}</p>
        <ul v-else class="job-list compact home-jobs">
          <li v-for="j in recentJobs" :key="jobKey(j)">
            <div class="job-info">
              <strong>{{ jobTypeLabel(j.type) }}</strong>
              <small>{{ j.message || jobStatusLabel(j.status) }}</small>
            </div>
            <n-tag size="small" round :type="jobStatusType(j.status)">
              {{ jobStatusLabel(j.status) }}
            </n-tag>
          </li>
        </ul>
      </div>
    </div>

  </section>
</template>

<style scoped>
.hero-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem 1rem;
  margin-bottom: 0.25rem;
}

.hero-line h2 {
  margin: 0;
  font-size: 1.35rem;
  font-weight: 650;
}

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.55rem;
}

.hero-busy {
  margin: 0.35rem 0 0;
}

button.stat {
  display: block;
  width: 100%;
  min-width: 0;
  text-align: left;
  font: inherit;
  color: inherit;
  cursor: pointer;
  appearance: none;
}

button.stat:hover {
  border-color: hsl(var(--primary) / 0.5);
}

.issue-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin: -0.5rem 0 1.25rem;
}

.issue {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  border: 1px solid hsl(var(--destructive) / 0.35);
  background: hsl(var(--destructive) / 0.08);
  color: hsl(var(--destructive));
  border-radius: 999px;
  padding: 0.35rem 0.75rem;
  font: inherit;
  font-size: 0.82rem;
  cursor: pointer;
}

.home-block {
  margin: 0 0 1.25rem;
  padding: 1rem 1.05rem;
  border-radius: 1rem;
  background: hsl(var(--card) / 0.55);
  border: 1px solid hsl(var(--border) / 0.55);
}

.home-block h3 {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 650;
}

.block-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}

.block-head h3,
.home-block > h3 {
  margin-bottom: 0.75rem;
}

.block-head h3 {
  margin-bottom: 0;
}

.text-link {
  border: 0;
  background: transparent;
  color: hsl(var(--primary));
  font: inherit;
  font-size: 0.82rem;
  cursor: pointer;
  padding: 0;
}

.home-split {
  display: grid;
  gap: 1rem;
  margin-bottom: 1.25rem;
}

.home-split .home-block {
  margin: 0;
}

.meter-list,
.platform-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.85rem;
}

.meter-head {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  font-size: 0.85rem;
}

.meter-head strong {
  font-variant-numeric: tabular-nums;
}

.meter {
  height: 8px;
  border-radius: 999px;
  background: hsl(var(--muted) / 0.7);
  overflow: hidden;
  margin: 0.35rem 0 0;
}

.meter > i {
  display: block;
  height: 100%;
  background: linear-gradient(90deg, hsl(var(--primary)), hsl(var(--accent)));
}

.meter.warn > i {
  background: hsl(var(--destructive));
}

.meter-list small {
  display: block;
  margin-top: 0.2rem;
  opacity: 0.55;
  font-size: 0.75rem;
}

.platform-row {
  display: block;
  width: 100%;
  text-align: left;
  border: 0;
  padding: 0.15rem 0;
  background: transparent;
  color: inherit;
  font: inherit;
  cursor: pointer;
}

.platform-row:hover strong {
  color: hsl(var(--primary));
}

.cover-strip {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(108px, 1fr));
  gap: 0.75rem;
}

.cover {
  position: relative;
  text-align: left;
  padding: 0;
  border: 0;
  cursor: pointer;
  font: inherit;
  color: inherit;
  border-radius: 1rem;
  overflow: hidden;
  background: hsl(var(--card) / 0.65);
  border: 1px solid hsl(var(--border) / 0.55);
}

.cover:hover {
  transform: translateY(-2px);
}

.meta {
  padding: 0.55rem 0.6rem 0.7rem;
  min-width: 0;
}

.meta strong {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  font-size: 0.82rem;
  line-height: 1.25;
  word-break: break-word;
}

.meta small {
  display: block;
  margin-top: 0.15rem;
  opacity: 0.6;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.source-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.home-jobs {
  margin-top: 0;
}

@media (min-width: 720px) {
  .cover-strip {
    grid-template-columns: repeat(auto-fill, minmax(124px, 1fr));
  }
}

@media (min-width: 900px) {
  .home-split {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
