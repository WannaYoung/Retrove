<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import {
  NButton,
  NCheckbox,
  NForm,
  NFormItem,
  NIcon,
  NModal,
  NRadio,
  NRadioGroup,
  NSelect,
  NTag,
} from "naive-ui";
import { ColorWandOutline } from "@vicons/ionicons5";
import { defaultScrapeFields, type ScrapeFields, type ScrapeWriteMode } from "../../api/client";
import { useAppStore } from "../app/composables/useApp";

const app = useAppStore();

const showModal = ref(false);
const scope = ref<"missing" | "all">("missing");
const writeMode = ref<ScrapeWriteMode>("update");
const fields = reactive<ScrapeFields>(defaultScrapeFields());

const canStart = computed(
  () =>
    !!app.scrapePlatformId &&
    (fields.title_en ||
      fields.description ||
      fields.developer ||
      fields.box ||
      fields.screenshot ||
      fields.title ||
      fields.logo),
);

watch(
  () => app.scrapePlatformOptions,
  (opts) => {
    if (!opts.length) {
      app.scrapePlatformId = "";
      return;
    }
    if (!opts.some((o) => o.value === app.scrapePlatformId)) {
      app.scrapePlatformId = opts[0]?.value || "";
    }
  },
  { immediate: true },
);

function openModal() {
  fields.title_zh = false;
  scope.value = "missing";
  writeMode.value = "update";
  showModal.value = true;
}

async function confirmScrape() {
  if (!canStart.value) return;
  fields.title_zh = false;
  showModal.value = false;
  await app.runScrape("batch", undefined, {
    platform_id: app.scrapePlatformId,
    only_missing: scope.value === "missing",
    write_mode: writeMode.value,
    fields: { ...fields, title_zh: false },
  });
}

const progressPct = computed(() => Math.round(Math.min(1, Math.max(0, app.jobProgress)) * 100));
</script>

<template>
  <section class="panel scrape">
    <p class="lead">{{ app.t("scrape.desc") }}</p>
    <div class="stat-row scrape-stats">
      <div class="stat">
        <span>{{ app.scrapeStatus?.missing_covers ?? "—" }}</span>
        <label>{{ app.t("scrape.missingCovers") }}</label>
      </div>
      <div class="stat wide">
        <label>{{ app.t("scrape.sourcesReady") }}</label>
        <div class="source-chips">
          <n-tag size="small" round :type="app.scrapeStatus?.screenscraper_configured ? 'success' : 'default'">
            {{ app.t("scrape.sourceSS") }}
          </n-tag>
          <n-tag size="small" round :type="app.scrapeStatus?.igdb_configured ? 'success' : 'default'">
            {{ app.t("scrape.sourceIGDB") }}
          </n-tag>
          <n-tag size="small" round :type="app.scrapeStatus?.steamgriddb_configured ? 'success' : 'default'">
            {{ app.t("scrape.sourceSGDB") }}
          </n-tag>
        </div>
      </div>
    </div>
    <p v-if="!app.anyScrapeConfigured" class="err">{{ app.t("scrape.notConfigured") }}</p>
    <p v-else-if="!app.scrapePlatformOptions.length" class="err">{{ app.t("scrape.noPlatforms") }}</p>
    <div class="actions scrape-actions">
      <button
        class="pill"
        :disabled="app.busy || !app.anyScrapeConfigured || !app.scrapePlatformOptions.length"
        @click="openModal"
      >
        <n-icon :component="ColorWandOutline" :size="18" />
        <span>{{ app.t("scrape.run") }}</span>
      </button>
    </div>

    <div v-if="app.busy" class="progress-block">
      <div class="progress-head">
        <p class="busy">{{ app.busyLabel || app.t("scrape.running") }}</p>
        <strong>{{ progressPct }}%</strong>
      </div>
      <div class="progress-bar" role="progressbar" :aria-valuenow="progressPct" aria-valuemin="0" aria-valuemax="100">
        <i :style="{ width: `${progressPct}%` }" />
      </div>
      <small>{{ app.t("scrape.progress") }}</small>
    </div>

    <div v-if="app.scrapeReport" class="report">
      <h3>{{ app.t("scrape.lastReport") }}</h3>
      <pre>{{ JSON.stringify(app.scrapeReport, null, 2) }}</pre>
    </div>
    <div class="report">
      <h3>{{ app.t("scrape.failures") }}</h3>
      <div v-if="!app.scrapeFailures.length" class="empty">{{ app.t("scrape.noFailures") }}</div>
      <ul v-else class="job-list">
        <li v-for="f in app.scrapeFailures" :key="`${f.game_id}-${f.filename}`">
          <div class="job-info">
            <strong>#{{ f.game_id }} {{ f.filename }}</strong>
            <small>{{ f.kind }}: {{ f.message }}</small>
          </div>
          <n-button size="tiny" secondary :disabled="app.busy" @click="app.runScrape('one', f.game_id)">
            {{ app.t("scrape.retryFailed") }}
          </n-button>
        </li>
      </ul>
      <div v-if="app.scrapeFailures.length" class="actions" style="margin-top: 0.75rem">
        <button class="pill ghost" :disabled="app.busy" @click="app.retryScrapeFailures">
          {{ app.t("scrape.retryFailed") }}
        </button>
      </div>
    </div>

    <n-modal
      v-model:show="showModal"
      preset="card"
      :title="app.t('scrape.modalTitle')"
      style="width: min(520px, calc(100vw - 2rem))"
      :mask-closable="!app.busy"
    >
      <n-form label-placement="top">
        <n-form-item :label="app.t('scrape.pickPlatform')">
          <n-select
            v-model:value="app.scrapePlatformId"
            :options="app.scrapePlatformOptions"
            :placeholder="app.t('scrape.pickPlatform')"
          />
        </n-form-item>
        <n-form-item :label="app.t('scrape.scope')">
          <n-radio-group v-model:value="scope" name="scrape-scope">
            <n-radio value="missing">{{ app.t("scrape.scopeMissing") }}</n-radio>
            <n-radio value="all">{{ app.t("scrape.scopeAll") }}</n-radio>
          </n-radio-group>
        </n-form-item>
        <n-form-item :label="app.t('scrape.writeMode')">
          <n-radio-group v-model:value="writeMode" name="scrape-write-mode">
            <n-radio value="update">{{ app.t("scrape.writeUpdate") }}</n-radio>
            <n-radio value="overwrite">{{ app.t("scrape.writeOverwrite") }}</n-radio>
          </n-radio-group>
        </n-form-item>
        <n-form-item :label="app.t('scrape.fields')">
          <div class="field-grid">
            <n-checkbox :checked="false" disabled>{{ app.t("scrape.fieldTitleZh") }}</n-checkbox>
            <n-checkbox v-model:checked="fields.title_en">{{ app.t("scrape.fieldTitleEn") }}</n-checkbox>
            <n-checkbox v-model:checked="fields.description">{{ app.t("scrape.fieldDescription") }}</n-checkbox>
            <n-checkbox v-model:checked="fields.developer">{{ app.t("scrape.fieldDeveloper") }}</n-checkbox>
            <n-checkbox v-model:checked="fields.box">{{ app.t("scrape.fieldBox") }}</n-checkbox>
            <n-checkbox v-model:checked="fields.screenshot">{{ app.t("scrape.fieldScreenshot") }}</n-checkbox>
            <n-checkbox v-model:checked="fields.title">{{ app.t("scrape.fieldTitle") }}</n-checkbox>
            <n-checkbox v-model:checked="fields.logo">{{ app.t("scrape.fieldLogo") }}</n-checkbox>
          </div>
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="modal-footer">
          <n-button @click="showModal = false">{{ app.t("settings.close") }}</n-button>
          <n-button type="primary" :disabled="!canStart" @click="confirmScrape">
            <template #icon>
              <n-icon :component="ColorWandOutline" />
            </template>
            {{ app.t("scrape.start") }}
          </n-button>
        </div>
      </template>
    </n-modal>
  </section>
</template>

<style scoped>
.scrape-actions {
  align-items: center;
}

.progress-block {
  margin-top: 1.25rem;
  padding: 1rem 1.05rem;
  border-radius: 1rem;
  border: 1px solid hsl(var(--border) / 0.65);
  background: hsl(var(--card) / 0.55);
}

.progress-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
}

.progress-head .busy {
  margin: 0;
  flex: 1;
  min-width: 0;
}

.progress-head strong {
  font-variant-numeric: tabular-nums;
  color: hsl(var(--primary));
}

.progress-bar {
  height: 10px;
  border-radius: 999px;
  background: hsl(var(--muted) / 0.7);
  overflow: hidden;
  margin: 0.65rem 0 0.35rem;
}

.progress-bar i {
  display: block;
  height: 100%;
  background: linear-gradient(90deg, hsl(var(--primary)), hsl(var(--accent)));
  transition: width 200ms ease;
}

.source-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin-top: 0.45rem;
}

.source-chips :deep(.n-tag) {
  font-size: 0.75rem;
  line-height: 1.2;
  height: auto;
  padding: 0.15rem 0.55rem;
}

.stat.wide > span {
  display: none;
}

.field-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.55rem 0.85rem;
  width: 100%;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.55rem;
}

@media (max-width: 480px) {
  .field-grid {
    grid-template-columns: 1fr;
  }
}
</style>
