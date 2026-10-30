<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { NButton, NInput, NModal, NSpin } from "naive-ui";
import {
  api,
  mediaURL,
  providerSupportsKind,
  type Game,
  type ScrapeApplyChoices,
  type ScrapeCandidate,
  type ScrapeSide,
} from "../../../api/client";
import { useAppStore } from "../../app/composables/useApp";

const props = defineProps<{ show: boolean; game: Game }>();
const emit = defineEmits<{
  "update:show": [boolean];
  saved: [Game];
}>();

const app = useAppStore();

const query = ref("");
const activeProvider = ref("");
const searching = ref(false);
const previewing = ref(false);
const saving = ref(false);
const error = ref("");
const results = ref<ScrapeCandidate[]>([]);
const selected = ref<ScrapeCandidate | null>(null);
const choices = reactive<ScrapeApplyChoices>({
  title_zh: "left",
  title_en: "left",
  description: "left",
  developer: "left",
  release_year: "left",
  box: "left",
  screenshot: "left",
  title: "left",
  logo: "left",
});

const providers = computed(() => {
  const list: { id: string; label: string }[] = [];
  if (app.scrapeStatus?.screenscraper_configured || app.settings?.screenscraper_configured) {
    list.push({ id: "screenscraper", label: app.t("scrape.sourceSS") });
  }
  if (app.scrapeStatus?.igdb_configured || app.settings?.igdb_configured) {
    list.push({ id: "igdb", label: app.t("scrape.sourceIGDB") });
  }
  if (app.scrapeStatus?.steamgriddb_configured || app.settings?.steamgriddb_configured) {
    list.push({ id: "steamgriddb", label: app.t("scrape.sourceSGDB") });
  }
  return list;
});

const canSave = computed(() => !!selected.value && !saving.value);
const showResults = computed(() => !selected.value && results.value.length > 0);

type CompareRow =
  | {
      kind: "text";
      key: keyof ScrapeApplyChoices;
      labelKey: string;
      left: string;
      right: string;
    }
  | {
      kind: "media";
      key: "box" | "screenshot" | "title" | "logo";
      labelKey: string;
      left: string;
      right: string;
    };

const rows = computed<CompareRow[]>(() => {
  const text: CompareRow[] = [
    {
      kind: "text",
      key: "title_zh",
      labelKey: "library.titleZh",
      left: props.game.title_zh || "",
      right: selected.value?.title_zh || "",
    },
    {
      kind: "text",
      key: "title_en",
      labelKey: "library.titleEn",
      left: props.game.title_en || "",
      right: selected.value?.title_en || "",
    },
    {
      kind: "text",
      key: "developer",
      labelKey: "library.developer",
      left: props.game.developer || "",
      right: selected.value?.developer || "",
    },
    {
      kind: "text",
      key: "release_year",
      labelKey: "library.releaseYear",
      left: props.game.release_year > 0 ? String(props.game.release_year) : "",
      right: selected.value?.release_year ? String(selected.value.release_year) : "",
    },
    {
      kind: "text",
      key: "description",
      labelKey: "library.description",
      left: props.game.description || "",
      right: selected.value?.description || "",
    },
  ];
  const provider = selected.value?.provider || activeProvider.value;
  const media: CompareRow[] = (
    [
      ["box", "scrape.fieldBox"],
      ["screenshot", "scrape.fieldScreenshot"],
      ["title", "scrape.fieldTitle"],
      ["logo", "scrape.fieldLogo"],
    ] as const
  )
    .filter(([key]) => !provider || providerSupportsKind(provider, key))
    .map(([key, labelKey]) => ({
      kind: "media" as const,
      key,
      labelKey,
      left: leftMedia(key),
      right: selected.value?.medias?.[key] || "",
    }));
  return [...text, ...media];
});

function leftMedia(kind: string) {
  const hit = props.game.media?.find((m) => m.kind === kind);
  return hit?.path || (kind === "box" ? props.game.cover_path : "");
}

function resultBox(item: ScrapeCandidate) {
  return item.medias?.box || "";
}

function resetState() {
  query.value = props.game.title_en || "";
  activeProvider.value = "";
  searching.value = false;
  previewing.value = false;
  saving.value = false;
  error.value = "";
  results.value = [];
  selected.value = null;
  Object.assign(choices, {
    title_zh: "left",
    title_en: "left",
    description: "left",
    developer: "left",
    release_year: "left",
    box: "left",
    screenshot: "left",
    title: "left",
    logo: "left",
  } satisfies ScrapeApplyChoices);
}

watch(
  () => props.show,
  (v) => {
    if (v) resetState();
  },
);

function setSide(key: keyof ScrapeApplyChoices, side: ScrapeSide) {
  if (!selected.value && side === "right") return;
  choices[key] = side;
}

async function runSearch(provider: string) {
  if (!query.value.trim()) {
    error.value = app.t("scrape.needEnglish");
    return;
  }
  activeProvider.value = provider;
  searching.value = true;
  error.value = "";
  results.value = [];
  selected.value = null;
  try {
    const res = await api.scrapeSearch(props.game.id, { provider, query: query.value.trim() });
    results.value = res.items || [];
    if (!results.value.length) error.value = app.t("library.manualNoResults");
  } catch (e) {
    error.value = e instanceof Error ? e.message : app.t("library.manualSearchFailed");
  } finally {
    searching.value = false;
  }
}

async function pickResult(item: ScrapeCandidate) {
  previewing.value = true;
  error.value = "";
  try {
    const res = await api.scrapePreview(props.game.id, {
      provider: item.provider || activeProvider.value,
      candidate_id: item.id,
      query: query.value.trim(),
    });
    selected.value = res.candidate;
    Object.assign(choices, res.choices);
  } catch (e) {
    selected.value = item;
    error.value = e instanceof Error ? e.message : app.t("library.manualPreviewFailed");
  } finally {
    previewing.value = false;
  }
}

function clearSelection() {
  selected.value = null;
  Object.assign(choices, {
    title_zh: "left",
    title_en: "left",
    description: "left",
    developer: "left",
    release_year: "left",
    box: "left",
    screenshot: "left",
    title: "left",
    logo: "left",
  } satisfies ScrapeApplyChoices);
}

async function save() {
  if (!selected.value) return;
  saving.value = true;
  error.value = "";
  try {
    const g = await api.scrapeApply(props.game.id, {
      candidate: selected.value,
      choices: { ...choices },
    });
    emit("saved", g);
    emit("update:show", false);
  } catch (e) {
    error.value = e instanceof Error ? e.message : app.t("library.manualApplyFailed");
  } finally {
    saving.value = false;
  }
}

function close() {
  emit("update:show", false);
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="app.t('library.manualScrape')"
    style="width: min(960px, calc(100vw - 1.5rem))"
    :mask-closable="false"
    :close-on-esc="!saving"
    @update:show="(v) => emit('update:show', v)"
  >
    <div class="manual-top">
      <n-input
        v-model:value="query"
        :placeholder="app.t('library.titleEn')"
        :disabled="searching || saving"
        @keyup.enter="providers[0] && runSearch(providers[0].id)"
      />
      <div class="provider-btns">
        <n-button
          v-for="p in providers"
          :key="p.id"
          size="small"
          :type="activeProvider === p.id ? 'primary' : 'default'"
          :loading="searching && activeProvider === p.id"
          :disabled="searching || saving || !query.trim()"
          @click="runSearch(p.id)"
        >
          {{ p.label }}
        </n-button>
      </div>
    </div>
    <p v-if="error" class="err">{{ error }}</p>

    <n-spin :show="searching || previewing">
      <!-- 搜索结果：仅未选中时显示 -->
      <ul v-if="showResults" class="result-list">
        <li
          v-for="item in results"
          :key="`${item.provider}-${item.id}`"
          @click="pickResult(item)"
        >
          <div class="result-thumb">
            <img v-if="resultBox(item)" :src="resultBox(item)" alt="" />
            <span v-else>—</span>
          </div>
          <div class="result-meta">
            <strong>{{ item.label || item.title_en || item.id }}</strong>
            <small v-if="item.developer">{{ item.developer }}</small>
          </div>
        </li>
      </ul>

      <div v-else-if="!selected && !searching" class="blank">
        {{ app.t("library.manualBlank") }}
      </div>

      <!-- 选中后：左右一一对应 -->
      <div v-if="selected" class="compare">
        <div class="compare-head">
          <span>{{ app.t("library.manualCurrent") }}</span>
          <span class="compare-head-right">
            {{ app.t("library.manualRemote") }}
            <button type="button" class="reselect" @click="clearSelection">
              {{ app.t("library.manualReselect") }}
            </button>
          </span>
        </div>
        <div v-for="row in rows" :key="row.key" class="compare-row">
          <div class="field-label">{{ app.t(row.labelKey) }}</div>
          <div class="pair">
            <button
              type="button"
              class="side-card"
              :class="{ active: choices[row.key] === 'left', media: row.kind === 'media' }"
              @click="setSide(row.key, 'left')"
            >
              <span class="side-tag">L</span>
              <template v-if="row.kind === 'text'">
                <p>{{ row.left || "—" }}</p>
              </template>
              <template v-else>
                <img v-if="row.left" :src="mediaURL(row.left)" alt="" />
                <span v-else class="empty-media">—</span>
              </template>
            </button>
            <button
              type="button"
              class="side-card"
              :class="{ active: choices[row.key] === 'right', media: row.kind === 'media' }"
              @click="setSide(row.key, 'right')"
            >
              <span class="side-tag">R</span>
              <template v-if="row.kind === 'text'">
                <p>{{ row.right || "—" }}</p>
              </template>
              <template v-else>
                <img v-if="row.right" :src="row.right" alt="" />
                <span v-else class="empty-media">—</span>
              </template>
            </button>
          </div>
        </div>
      </div>
    </n-spin>

    <template #footer>
      <div class="modal-footer">
        <n-button :disabled="saving" @click="close">{{ app.t("settings.close") }}</n-button>
        <n-button type="primary" :disabled="!canSave" :loading="saving" @click="save">
          {{ app.t("library.save") }}
        </n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.manual-top {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
  align-items: center;
  margin-bottom: 0.85rem;
}

.manual-top :deep(.n-input) {
  flex: 1 1 220px;
  min-width: 0;
}

.provider-btns {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.blank {
  opacity: 0.55;
  padding: 1.5rem 0.5rem;
  text-align: center;
}

.result-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.45rem;
  max-height: min(52vh, 560px);
  overflow: auto;
}

.result-list li {
  display: grid;
  grid-template-columns: 56px 1fr;
  gap: 0.65rem;
  align-items: center;
  padding: 0.45rem 0.55rem;
  border-radius: 0.75rem;
  border: 1px solid hsl(var(--border) / 0.65);
  cursor: pointer;
  background: hsl(var(--card) / 0.45);
}

.result-list li:hover {
  border-color: hsl(var(--primary) / 0.55);
}

.result-thumb {
  width: 56px;
  height: 72px;
  border-radius: 0.45rem;
  overflow: hidden;
  background: hsl(var(--muted) / 0.4);
  display: grid;
  place-items: center;
}

.result-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.result-thumb span {
  opacity: 0.35;
  font-size: 0.75rem;
}

.result-meta strong {
  display: block;
  font-size: 0.9rem;
}

.result-meta small {
  opacity: 0.65;
}

.compare {
  max-height: min(58vh, 640px);
  overflow: auto;
}

.compare-head {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.55rem;
  margin: 0 0 0.75rem;
  padding-left: 0;
  font-size: 0.85rem;
  font-weight: 600;
  opacity: 0.8;
}

.compare-head-right {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.reselect {
  border: 0;
  background: transparent;
  color: hsl(var(--primary));
  font: inherit;
  font-size: 0.78rem;
  font-weight: 500;
  cursor: pointer;
  padding: 0;
}

.compare-row {
  margin-bottom: 0.65rem;
}

.field-label {
  font-size: 0.78rem;
  opacity: 0.65;
  margin-bottom: 0.3rem;
}

.pair {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.55rem;
}

.side-card {
  width: 100%;
  text-align: left;
  border: 1px solid hsl(var(--border) / 0.65);
  border-radius: 0.75rem;
  background: hsl(var(--card) / 0.4);
  color: inherit;
  font: inherit;
  padding: 0.55rem 0.65rem 0.55rem 2rem;
  position: relative;
  cursor: pointer;
  min-height: 2.6rem;
}

.side-card.active {
  border-color: hsl(var(--primary));
  background: hsl(var(--primary) / 0.08);
}

.side-card p {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 0.88rem;
  line-height: 1.35;
  max-height: 7.5rem;
  overflow: auto;
}

.side-tag {
  position: absolute;
  left: 0.45rem;
  top: 0.55rem;
  font-size: 0.7rem;
  font-weight: 700;
  opacity: 0.55;
}

.side-card.media {
  min-height: 96px;
  display: grid;
  place-items: center;
  padding: 0.45rem 0.45rem 0.45rem 1.6rem;
}

.side-card.media img {
  max-width: 100%;
  max-height: 120px;
  object-fit: contain;
  border-radius: 0.4rem;
}

.empty-media {
  opacity: 0.4;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.55rem;
}
</style>
