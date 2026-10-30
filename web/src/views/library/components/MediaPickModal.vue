<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { NButton, NModal, NSpin } from "naive-ui";
import {
  api,
  type Game,
  type MediaKind,
  type ScrapeMediaImage,
} from "../../../api/client";
import { useAppStore } from "../../app/composables/useApp";

const props = defineProps<{
  show: boolean;
  gameId: number;
  kind: MediaKind;
  provider: string;
  providerLabel: string;
  kindLabel: string;
}>();

const emit = defineEmits<{
  "update:show": [boolean];
  picked: [Game];
}>();

const app = useAppStore();
const loading = ref(false);
const saving = ref(false);
const error = ref("");
const items = ref<ScrapeMediaImage[]>([]);
const label = ref("");
const selectedURL = ref("");

const title = computed(
  () => `${props.providerLabel} · ${props.kindLabel}`,
);

watch(
  () => props.show,
  (v) => {
    if (v) void load();
    else {
      items.value = [];
      selectedURL.value = "";
      error.value = "";
    }
  },
);

async function load() {
  loading.value = true;
  error.value = "";
  selectedURL.value = "";
  items.value = [];
  try {
    const res = await api.scrapeMediaBrowse(props.gameId, {
      provider: props.provider,
      kind: props.kind,
    });
    items.value = res.items || [];
    label.value = res.game_label || "";
    if (!items.value.length) {
      error.value = app.t("library.mediaPickEmpty");
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : app.t("library.mediaPickFailed");
  } finally {
    loading.value = false;
  }
}

async function confirm() {
  if (!selectedURL.value || saving.value) return;
  saving.value = true;
  error.value = "";
  try {
    const g = await api.scrapeMediaPick(props.gameId, {
      provider: props.provider,
      kind: props.kind,
      url: selectedURL.value,
    });
    emit("picked", g);
    emit("update:show", false);
  } catch (e) {
    error.value = e instanceof Error ? e.message : app.t("library.mediaPickApplyFailed");
  } finally {
    saving.value = false;
  }
}

function close() {
  if (saving.value) return;
  emit("update:show", false);
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="title"
    style="width: min(720px, 94vw)"
    :mask-closable="!saving"
    @update:show="(v: boolean) => !v && close()"
  >
    <p v-if="label" class="hint">{{ label }}</p>
    <n-spin :show="loading">
      <p v-if="error" class="err">{{ error }}</p>
      <div v-else class="grid">
        <button
          v-for="(it, i) in items"
          :key="it.url + i"
          type="button"
          class="tile"
          :class="{ on: selectedURL === it.url }"
          @click="selectedURL = it.url"
        >
          <img :src="it.thumb || it.url" alt="" loading="lazy" />
        </button>
      </div>
    </n-spin>
    <template #footer>
      <div class="footer">
        <n-button quaternary :disabled="saving" @click="close">{{ app.t("settings.close") }}</n-button>
        <n-button
          type="primary"
          :disabled="!selectedURL"
          :loading="saving"
          @click="confirm"
        >
          {{ app.t("library.mediaPickApply") }}
        </n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.hint {
  margin: 0 0 0.75rem;
  font-size: 0.85rem;
  opacity: 0.65;
}
.err {
  margin: 0.5rem 0;
  color: hsl(var(--destructive));
  font-size: 0.9rem;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 0.55rem;
  max-height: min(52vh, 420px);
  overflow: auto;
  padding: 0.15rem;
}
.tile {
  border: 2px solid transparent;
  border-radius: 0.65rem;
  padding: 0;
  overflow: hidden;
  cursor: pointer;
  background: hsl(var(--muted) / 0.35);
  aspect-ratio: 1;
}
.tile.on {
  border-color: hsl(var(--primary));
}
.tile img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
