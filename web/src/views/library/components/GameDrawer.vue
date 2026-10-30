<script setup lang="ts">
import { computed, ref } from "vue";
import { NButton, NForm, NFormItem, NIcon, NInput, NInputNumber, NSwitch } from "naive-ui";
import {
  CloseOutline,
  ColorWandOutline,
  CloudUploadOutline,
  LockClosedOutline,
  SaveOutline,
} from "@vicons/ionicons5";
import {
  mediaURL,
  providerSupportsKind,
  type Game,
  type MediaKind,
} from "../../../api/client";
import { tileLetter, useAppStore } from "../../app/composables/useApp";
import ManualScrapeModal from "./ManualScrapeModal.vue";
import MediaPickModal from "./MediaPickModal.vue";

const app = useAppStore();
const fileInputs = ref<Record<string, HTMLInputElement | null>>({});

const mediaKinds: { kind: MediaKind; labelKey: string }[] = [
  { kind: "box", labelKey: "scrape.fieldBox" },
  { kind: "screenshot", labelKey: "scrape.fieldScreenshot" },
  { kind: "title", labelKey: "scrape.fieldTitle" },
  { kind: "logo", labelKey: "scrape.fieldLogo" },
];

const canManual = computed(
  () => app.anyScrapeConfigured && !app.busy && !!app.selected,
);

const sourceButtons = computed(() => {
  const list: { id: string; short: string; labelKey: string }[] = [];
  if (app.scrapeStatus?.screenscraper_configured || app.settings?.screenscraper_configured) {
    list.push({ id: "screenscraper", short: "SS", labelKey: "scrape.sourceSS" });
  }
  if (app.scrapeStatus?.steamgriddb_configured || app.settings?.steamgriddb_configured) {
    list.push({ id: "steamgriddb", short: "SGDB", labelKey: "scrape.sourceSGDB" });
  }
  if (app.scrapeStatus?.igdb_configured || app.settings?.igdb_configured) {
    list.push({ id: "igdb", short: "IGDB", labelKey: "scrape.sourceIGDB" });
  }
  return list;
});

const pickOpen = ref(false);
const pickKind = ref<MediaKind>("box");
const pickProvider = ref("screenscraper");
const pickProviderLabel = ref("");
const pickKindLabel = ref("");

function sourcesFor(kind: MediaKind) {
  return sourceButtons.value.filter((s) => providerSupportsKind(s.id, kind));
}

function openPick(kind: MediaKind, provider: string, labelKey: string) {
  pickKind.value = kind;
  pickProvider.value = provider;
  pickProviderLabel.value = app.t(labelKey);
  pickKindLabel.value = app.t(
    mediaKinds.find((m) => m.kind === kind)?.labelKey || "library.media",
  );
  pickOpen.value = true;
}

async function onPicked(g: Game) {
  await app.onManualScrapeSaved(g);
}

function triggerUpload(kind: MediaKind) {
  fileInputs.value[kind]?.click();
}

async function onFileChange(kind: MediaKind, ev: Event) {
  const input = ev.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file) return;
  try {
    await app.uploadMedia(kind, file);
  } catch (e) {
    app.error = e instanceof Error ? e.message : "upload failed";
  }
}
</script>

<template>
  <aside v-if="app.selected" class="drawer">
    <button class="drawer-close" @click="app.closeGame()">
      <n-icon :component="CloseOutline" :size="18" />
      <span>{{ app.t("library.close") }}</span>
    </button>

    <div class="drawer-art" :data-tone="app.selected.platform_id.length % 5">
      <img v-if="app.mediaPath('box')" :src="mediaURL(app.mediaPath('box'))" alt="" />
      <span v-else>{{ tileLetter(app.selected.title_zh || app.selected.filename) }}</span>
    </div>

    <p class="meta-line">
      {{ app.selected.platform_name }}
      <span v-if="app.selected.scrape_id"> · {{ app.selected.scrape_id }}</span>
    </p>

    <n-form label-placement="top">
      <n-form-item :label="app.t('library.titleZh')">
        <n-input v-model:value="app.editTitleZh" />
      </n-form-item>
      <n-form-item :label="app.t('library.lockTitle')">
        <div class="lock-row">
          <n-icon :component="LockClosedOutline" :size="16" />
          <n-switch v-model:value="app.editLocked" />
        </div>
      </n-form-item>
      <n-form-item :label="app.t('library.titleEn')">
        <n-input v-model:value="app.editTitleEn" />
      </n-form-item>
      <n-form-item :label="app.t('library.developer')">
        <n-input v-model:value="app.editDeveloper" />
      </n-form-item>
      <n-form-item :label="app.t('library.releaseYear')">
        <n-input-number v-model:value="app.editReleaseYear" :min="1970" :max="2100" clearable style="width: 100%" />
      </n-form-item>
      <n-form-item :label="app.t('library.description')">
        <n-input v-model:value="app.editDescription" type="textarea" :rows="5" />
      </n-form-item>
      <n-form-item :label="app.t('library.lockDescription')">
        <div class="lock-row">
          <n-icon :component="LockClosedOutline" :size="16" />
          <n-switch v-model:value="app.editDescLocked" />
        </div>
      </n-form-item>

      <div class="media-block">
        <h3>{{ app.t("library.media") }}</h3>
        <div class="media-grid">
          <div v-for="m in mediaKinds" :key="m.kind" class="media-item">
            <div class="media-thumb">
              <img v-if="app.mediaPath(m.kind)" :src="mediaURL(app.mediaPath(m.kind))" alt="" />
              <span v-else class="empty">{{ app.t(m.labelKey) }}</span>
            </div>
            <div class="media-actions">
              <small>{{ app.t(m.labelKey) }}</small>
              <div class="media-btns">
                <n-button
                  v-for="s in sourcesFor(m.kind)"
                  :key="s.id"
                  size="tiny"
                  quaternary
                  :disabled="!app.selected?.title_en || app.busy"
                  :title="app.t(s.labelKey)"
                  @click="openPick(m.kind, s.id, s.labelKey)"
                >
                  {{ s.short }}
                </n-button>
                <n-button
                  size="tiny"
                  secondary
                  :loading="app.mediaUploading === m.kind"
                  @click="triggerUpload(m.kind)"
                >
                  <template #icon>
                    <n-icon :component="CloudUploadOutline" />
                  </template>
                  {{ app.t("library.upload") }}
                </n-button>
              </div>
              <input
                :ref="(el) => (fileInputs[m.kind] = el as HTMLInputElement | null)"
                type="file"
                accept="image/*"
                hidden
                @change="onFileChange(m.kind, $event)"
              />
            </div>
          </div>
        </div>
      </div>

      <p class="file-line">{{ app.t("library.file") }}: {{ app.selected.rom_path || app.selected.filename }}</p>
      <div class="drawer-actions">
        <n-button type="primary" block @click="app.saveGame">
          <template #icon>
            <n-icon :component="SaveOutline" />
          </template>
          {{ app.t("library.save") }}
        </n-button>
        <n-button secondary block :disabled="!canManual" @click="app.openManualScrape">
          <template #icon>
            <n-icon :component="ColorWandOutline" />
          </template>
          {{ app.t("library.manualScrape") }}
        </n-button>
      </div>
    </n-form>

    <ManualScrapeModal
      v-model:show="app.manualScrapeOpen"
      :game="app.selected"
      @saved="app.onManualScrapeSaved"
    />
    <MediaPickModal
      v-if="app.selected"
      v-model:show="pickOpen"
      :game-id="app.selected.id"
      :kind="pickKind"
      :provider="pickProvider"
      :provider-label="pickProviderLabel"
      :kind-label="pickKindLabel"
      @picked="onPicked"
    />
  </aside>
</template>

<style scoped>
.drawer {
  position: fixed;
  inset: 0;
  z-index: 40;
  width: auto;
  padding: 1rem;
  overflow: auto;
  background: hsl(var(--popover) / 0.98);
  backdrop-filter: blur(18px);
}

.drawer-art {
  border-radius: 1.1rem;
  margin: 0.5rem 0 0.75rem;
  aspect-ratio: 16 / 10;
  font-size: 2rem;
}

.drawer-close {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  border: 0;
  cursor: pointer;
  font: inherit;
  background: transparent;
  color: inherit;
  opacity: 0.7;
  margin-bottom: 0.35rem;
  padding: 0.35rem 0.15rem;
}

.meta-line {
  margin: 0 0 0.85rem;
  font-size: 0.85rem;
  opacity: 0.65;
}

.drawer-actions {
  display: grid;
  gap: 0.55rem;
}

.lock-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.media-block {
  margin: 0.35rem 0 1rem;
}

.media-block h3 {
  margin: 0 0 0.65rem;
  font-size: 0.95rem;
}

.media-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.65rem;
}

.media-item {
  border: 1px solid hsl(var(--border) / 0.6);
  border-radius: 0.85rem;
  overflow: hidden;
  background: hsl(var(--card) / 0.45);
}

.media-thumb {
  aspect-ratio: 4 / 3;
  display: grid;
  place-items: center;
  background: hsl(var(--muted) / 0.35);
}

.media-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.media-thumb .empty {
  opacity: 0.45;
  font-size: 0.78rem;
  padding: 0.5rem;
  text-align: center;
}

.media-actions {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 0.35rem;
  padding: 0.4rem 0.5rem;
}

.media-actions small {
  opacity: 0.65;
  font-size: 0.72rem;
}

.media-btns {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.25rem;
}

.file-line {
  margin: 0 0 0.75rem;
  font-size: 0.78rem;
  opacity: 0.55;
  word-break: break-all;
}

@media (min-width: 1024px) {
  .drawer {
    position: relative;
    inset: auto;
    height: 100%;
    width: auto;
    min-width: 0;
    border-left: 1px solid hsl(var(--border) / 0.7);
    background: hsl(var(--popover) / 0.92);
  }
}
</style>
