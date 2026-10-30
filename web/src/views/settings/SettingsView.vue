<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { NButton, NForm, NFormItem, NIcon, NInput, NModal, NSelect, NTabPane, NTabs, NTag } from "naive-ui";
import {
  CloudDownloadOutline,
  LanguageOutline,
  RefreshOutline,
  SaveOutline,
  ScanOutline,
} from "@vicons/ionicons5";
import { isSettingsTab, type SettingsTab } from "../../router";
import { useAppStore, type LocaleId, type ThemeId } from "../app/composables/useApp";

const app = useAppStore();
const route = useRoute();
const router = useRouter();

const settingsTab = computed<SettingsTab>({
  get() {
    return isSettingsTab(route.params.tab) ? route.params.tab : "appearance";
  },
  set(tab) {
    void router.push({ name: "settings", params: { tab } });
  },
});
</script>

<template>
  <section class="panel settings">
    <n-tabs v-model:value="settingsTab" type="segment" animated class="settings-tabs">
      <n-tab-pane name="appearance" :tab="app.t('settings.appearance')">
        <div class="settings-block">
          <div class="settings-field">
            <span>{{ app.t("app.language") }}</span>
            <n-select
              :value="app.localeId"
              :options="app.languageOptions"
              class="settings-select"
              @update:value="(v) => app.setLocale(v as LocaleId)"
            />
          </div>
          <div class="settings-field">
            <span>{{ app.t("app.theme") }}</span>
            <n-select
              :value="app.themeId"
              :options="app.themeSelectOptions"
              class="settings-select wide"
              filterable
              @update:value="(v) => app.setTheme(v as ThemeId)"
            />
          </div>
        </div>
      </n-tab-pane>

      <n-tab-pane name="tasks" :tab="app.t('settings.tasks')">
        <div class="settings-block">
          <p class="settings-note">{{ app.t("scan.desc") }}</p>
          <div class="actions">
            <button class="pill" :disabled="app.busy" @click="app.runJob('scan')">
              <n-icon :component="ScanOutline" :size="18" />
              <span>{{ app.t("scan.runScan") }}</span>
            </button>
            <button class="pill" :disabled="app.busy" @click="app.runJob('import')">
              <n-icon :component="CloudDownloadOutline" :size="18" />
              <span>{{ app.t("scan.runImport") }}</span>
            </button>
          </div>
          <p v-if="app.busy" class="busy">{{ app.busyLabel }}</p>
          <details v-if="app.lastReport" class="report-fold">
            <summary>{{ app.t("scan.lastReport") }}</summary>
            <pre>{{ JSON.stringify(app.lastReport, null, 2) }}</pre>
          </details>

          <div class="jobs-section">
            <h3 class="jobs-heading">{{ app.t("jobs.recent") }}</h3>
            <div v-if="!app.jobs.length" class="empty">{{ app.t("jobs.empty") }}</div>
            <ul v-else class="job-list compact">
              <li v-for="j in app.jobs" :key="j.id">
                <div class="job-info">
                  <strong>#{{ j.id }} {{ j.type }}</strong>
                  <small>{{ j.message || j.status }}</small>
                </div>
                <n-tag size="small" round>{{ j.status }}</n-tag>
              </li>
            </ul>
          </div>
        </div>
      </n-tab-pane>

      <n-tab-pane name="translate" :tab="app.t('settings.translate')">
        <div class="settings-block">
          <p class="settings-note">{{ app.t("settings.llmHint") }}</p>
          <n-form label-placement="top" class="llm-form">
            <n-form-item :label="app.t('settings.llmBaseUrl')">
              <n-input v-model:value="app.llmBaseUrl" />
            </n-form-item>
            <n-form-item :label="app.t('settings.llmApiKey')">
              <n-input
                v-model:value="app.llmApiKey"
                type="password"
                show-password-on="click"
                :placeholder="app.settings?.providers?.llm?.api_key_set ? '••••••••' : ''"
              />
            </n-form-item>
            <n-form-item :label="app.t('settings.llmModel')">
              <div class="model-row">
                <n-select
                  v-model:value="app.llmModel"
                  :options="app.llmModels"
                  filterable
                  tag
                  :placeholder="app.t('settings.llmModelPlaceholder')"
                  class="model-select"
                />
                <n-button secondary :loading="app.llmFetchingModels" @click="app.fetchLLMModels">
                  <template #icon>
                    <n-icon :component="RefreshOutline" />
                  </template>
                  {{ app.t("settings.llmFetchModels") }}
                </n-button>
              </div>
            </n-form-item>
          </n-form>
          <div class="actions">
            <n-button type="primary" :loading="app.llmSaving" @click="app.saveLLM">
              <template #icon>
                <n-icon :component="SaveOutline" />
              </template>
              {{ app.t("settings.llmSave") }}
            </n-button>
            <n-button secondary :loading="app.llmTesting" @click="app.testLLM">
              {{ app.t("settings.test") }}
            </n-button>
          </div>
          <p v-if="app.llmSaved" class="busy">{{ app.t("settings.saved") }}</p>
          <p
            v-if="app.llmTestResult"
            class="provider-test-msg"
            :class="{ ok: app.llmTestResult.ok, fail: !app.llmTestResult.ok }"
          >
            {{ app.llmTestResult.ok ? app.t("settings.testOk") : app.t("settings.testFail") }}
            <span v-if="!app.llmTestResult.ok"> — {{ app.llmTestResult.message }}</span>
          </p>

          <div class="jobs-section">
            <h3 class="jobs-heading">{{ app.t("settings.llmRomPlatforms") }}</h3>
            <p v-if="!app.llmConfigured" class="settings-note">{{ app.t("settings.llmNeedConfig") }}</p>
            <div v-else-if="!app.romPlatforms.length" class="empty">{{ app.t("settings.llmNoPlatforms") }}</div>
            <div v-else class="provider-list">
              <div v-for="p in app.romPlatforms" :key="p.id" class="provider-row">
                <div>
                  <strong>{{ p.name }}</strong>
                  <n-tag size="tiny" round class="provider-tag">
                    {{ app.t("settings.llmRomCount", { n: p.rom_count ?? 0 }) }}
                  </n-tag>
                  <n-tag v-if="(p.pending_en ?? 0) > 0" size="tiny" round type="warning" class="provider-tag">
                    {{ app.t("settings.llmPendingEn", { n: p.pending_en ?? 0 }) }}
                  </n-tag>
                </div>
                <div class="provider-actions">
                  <n-button
                    type="primary"
                    secondary
                    size="small"
                    :disabled="app.busy"
                    :loading="app.translatePlatformId === p.id"
                    @click="app.runTranslatePlatform(p.id)"
                  >
                    <template #icon>
                      <n-icon :component="LanguageOutline" />
                    </template>
                    {{ app.t("settings.llmTranslate") }}
                  </n-button>
                </div>
              </div>
            </div>
            <p v-if="app.busy && app.translatePlatformId" class="busy">{{ app.busyLabel }}</p>
            <details v-if="app.lastReport && settingsTab === 'translate'" class="report-fold">
              <summary>{{ app.t("scan.lastReport") }}</summary>
              <pre>{{ JSON.stringify(app.lastReport, null, 2) }}</pre>
            </details>
          </div>
        </div>
      </n-tab-pane>

      <n-tab-pane name="providers" :tab="app.t('settings.providers')">
        <div class="settings-block">
          <p class="settings-note">{{ app.t("settings.providersHint") }}</p>
          <div class="provider-list">
            <div class="provider-row">
              <div>
                <strong>{{ app.t("settings.ssTitle") }}</strong>
                <n-tag
                  size="tiny"
                  round
                  :type="app.settings?.screenscraper_configured ? 'success' : 'default'"
                  class="provider-tag"
                >
                  {{ app.settings?.screenscraper_configured ? app.t("settings.statusOn") : app.t("settings.statusOff") }}
                </n-tag>
              </div>
              <div class="provider-actions">
                <n-button
                  v-if="app.settings?.screenscraper_configured"
                  secondary
                  size="small"
                  :loading="app.providerTesting === 'ss'"
                  @click="app.testProvider('ss')"
                >
                  {{ app.t("settings.test") }}
                </n-button>
                <n-button secondary size="small" @click="app.openProvider('ss')">{{ app.t("settings.configure") }}</n-button>
              </div>
            </div>
            <p
              v-if="app.providerTestResult?.id === 'ss'"
              class="provider-test-msg"
              :class="{ ok: app.providerTestResult.ok, fail: !app.providerTestResult.ok }"
            >
              {{ app.providerTestResult.ok ? app.t("settings.testOk") : app.t("settings.testFail") }}
              <span v-if="!app.providerTestResult.ok"> — {{ app.providerTestResult.message }}</span>
            </p>
            <div class="provider-row">
              <div>
                <strong>{{ app.t("settings.igdbTitle") }}</strong>
                <n-tag
                  size="tiny"
                  round
                  :type="app.settings?.igdb_configured ? 'success' : 'default'"
                  class="provider-tag"
                >
                  {{ app.settings?.igdb_configured ? app.t("settings.statusOn") : app.t("settings.statusOff") }}
                </n-tag>
              </div>
              <div class="provider-actions">
                <n-button
                  v-if="app.settings?.igdb_configured"
                  secondary
                  size="small"
                  :loading="app.providerTesting === 'igdb'"
                  @click="app.testProvider('igdb')"
                >
                  {{ app.t("settings.test") }}
                </n-button>
                <n-button secondary size="small" @click="app.openProvider('igdb')">{{ app.t("settings.configure") }}</n-button>
              </div>
            </div>
            <p
              v-if="app.providerTestResult?.id === 'igdb'"
              class="provider-test-msg"
              :class="{ ok: app.providerTestResult.ok, fail: !app.providerTestResult.ok }"
            >
              {{ app.providerTestResult.ok ? app.t("settings.testOk") : app.t("settings.testFail") }}
              <span v-if="!app.providerTestResult.ok"> — {{ app.providerTestResult.message }}</span>
            </p>
            <div class="provider-row">
              <div>
                <strong>{{ app.t("settings.sgdbTitle") }}</strong>
                <n-tag
                  size="tiny"
                  round
                  :type="app.settings?.steamgriddb_configured ? 'success' : 'default'"
                  class="provider-tag"
                >
                  {{
                    app.settings?.steamgriddb_configured ? app.t("settings.statusOn") : app.t("settings.statusOff")
                  }}
                </n-tag>
              </div>
              <div class="provider-actions">
                <n-button
                  v-if="app.settings?.steamgriddb_configured"
                  secondary
                  size="small"
                  :loading="app.providerTesting === 'sgdb'"
                  @click="app.testProvider('sgdb')"
                >
                  {{ app.t("settings.test") }}
                </n-button>
                <n-button secondary size="small" @click="app.openProvider('sgdb')">{{ app.t("settings.configure") }}</n-button>
              </div>
            </div>
            <p
              v-if="app.providerTestResult?.id === 'sgdb'"
              class="provider-test-msg"
              :class="{ ok: app.providerTestResult.ok, fail: !app.providerTestResult.ok }"
            >
              {{ app.providerTestResult.ok ? app.t("settings.testOk") : app.t("settings.testFail") }}
              <span v-if="!app.providerTestResult.ok"> — {{ app.providerTestResult.message }}</span>
            </p>
          </div>
        </div>
      </n-tab-pane>
    </n-tabs>

    <n-modal
      :show="!!app.providerModal"
      preset="card"
      :title="
        app.providerModal === 'ss'
          ? app.t('settings.ssTitle')
          : app.providerModal === 'igdb'
            ? app.t('settings.igdbTitle')
            : app.t('settings.sgdbTitle')
      "
      style="width: min(480px, calc(100vw - 2rem))"
      :mask-closable="true"
      @update:show="(v) => !v && (app.providerModal = null)"
    >
      <p class="lead modal-help">
        {{
          app.providerModal === "ss"
            ? app.t("settings.ssHelp")
            : app.providerModal === "igdb"
              ? app.t("settings.igdbHelp")
              : app.t("settings.sgdbHelp")
        }}
      </p>
      <n-form v-if="app.providerModal === 'ss'" label-placement="top">
        <n-form-item :label="app.t('settings.ssDevId')">
          <n-input v-model:value="app.ssDevId" />
        </n-form-item>
        <n-form-item :label="app.t('settings.ssDevPassword')">
          <n-input v-model:value="app.ssDevPassword" type="password" show-password-on="click" />
        </n-form-item>
        <n-form-item :label="app.t('settings.ssUser')">
          <n-input v-model:value="app.ssUser" />
        </n-form-item>
        <n-form-item :label="app.t('settings.ssUserPassword')">
          <n-input v-model:value="app.ssUserPassword" type="password" show-password-on="click" />
        </n-form-item>
        <n-form-item :label="app.t('settings.ssSoftname')">
          <n-input v-model:value="app.ssSoftname" />
        </n-form-item>
      </n-form>
      <n-form v-else-if="app.providerModal === 'igdb'" label-placement="top">
        <n-form-item :label="app.t('settings.igdbClientId')">
          <n-input v-model:value="app.igdbClientId" />
        </n-form-item>
        <n-form-item :label="app.t('settings.igdbClientSecret')">
          <n-input v-model:value="app.igdbClientSecret" type="password" show-password-on="click" />
        </n-form-item>
      </n-form>
      <n-form v-else-if="app.providerModal === 'sgdb'" label-placement="top">
        <n-form-item :label="app.t('settings.sgdbApiKey')">
          <n-input v-model:value="app.sgdbApiKey" type="password" show-password-on="click" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="modal-footer">
          <n-button @click="app.providerModal = null">{{ app.t("settings.close") }}</n-button>
          <n-button type="primary" :loading="app.providerSaving" @click="app.saveProvider">
            <template #icon>
              <n-icon :component="SaveOutline" />
            </template>
            {{ app.t("settings.save") }}
          </n-button>
        </div>
        <p v-if="app.providerSaved" class="busy">{{ app.t("settings.saved") }}</p>
      </template>
    </n-modal>
  </section>
</template>

<style scoped>
.settings-tabs {
  max-width: 820px;
}

.settings-tabs :deep(.n-tabs-tab-pad) {
  padding: 0 0.35rem;
}

.settings-tabs :deep(.n-tabs-rail) {
  background: hsl(var(--muted) / 0.75);
}

.settings-tabs :deep(.n-tabs-tab) {
  color: hsl(var(--muted-foreground));
}

.settings-tabs :deep(.n-tabs-tab--active) {
  color: hsl(var(--foreground));
}

.settings-tabs :deep(.n-tabs-capsule) {
  background: hsl(var(--card));
}

.settings-block :deep(.n-base-selection),
.settings-block :deep(.n-input) {
  --n-color: hsl(var(--card)) !important;
  --n-color-active: hsl(var(--card)) !important;
  --n-text-color: hsl(var(--foreground)) !important;
  --n-border: 1px solid hsl(var(--border)) !important;
  --n-border-active: 1px solid hsl(var(--ring)) !important;
  --n-border-hover: 1px solid hsl(var(--ring)) !important;
  --n-placeholder-color: hsl(var(--muted-foreground)) !important;
}

.settings-block {
  display: grid;
  gap: 0.9rem;
  padding-top: 0.85rem;
}

.settings-field {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
  padding: 0.85rem 1rem;
  border-radius: 0.95rem;
  background: hsl(var(--card) / 0.55);
  border: 1px solid hsl(var(--border) / 0.5);
}

.settings-field span {
  opacity: 0.75;
  font-size: 0.92rem;
}

.settings-note {
  margin: 0;
  opacity: 0.7;
  font-size: 0.9rem;
  line-height: 1.45;
}

.settings-select {
  width: 180px;
  max-width: 100%;
}

.settings-select.wide {
  width: 220px;
}

.llm-form {
  display: grid;
  gap: 0.15rem;
}

.model-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.55rem;
  width: 100%;
  align-items: center;
}

.model-select {
  flex: 1 1 180px;
  min-width: 0;
}

.jobs-section {
  display: grid;
  gap: 0.65rem;
  padding-top: 0.35rem;
}

.jobs-heading {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 600;
  opacity: 0.85;
}

.provider-list {
  display: grid;
  gap: 0.55rem;
}

.provider-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  padding: 0.85rem 1rem;
  border-radius: 0.95rem;
  background: hsl(var(--card) / 0.55);
  border: 1px solid hsl(var(--border) / 0.5);
}

.provider-row > div {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  min-width: 0;
}

.provider-tag {
  flex-shrink: 0;
}

.provider-actions {
  display: flex;
  flex-shrink: 0;
  gap: 0.45rem;
  align-items: center;
}

.provider-test-msg {
  margin: -0.15rem 0 0.35rem;
  padding: 0 0.25rem;
  font-size: 0.82rem;
  line-height: 1.35;
}

.provider-test-msg.ok {
  color: hsl(var(--primary));
}

.provider-test-msg.fail {
  color: hsl(var(--destructive));
}

.report-fold {
  border-radius: 0.95rem;
  border: 1px solid hsl(var(--border) / 0.5);
  background: hsl(var(--card) / 0.45);
  padding: 0.65rem 0.9rem;
}

.report-fold summary {
  cursor: pointer;
  opacity: 0.8;
  font-size: 0.9rem;
}

.report-fold pre {
  margin: 0.65rem 0 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 0.82rem;
  opacity: 0.85;
}

.modal-help {
  margin-bottom: 0.85rem;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.55rem;
}
</style>
