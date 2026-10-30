<script setup lang="ts">
import { NButton, NIcon, NInput, NSelect, NTag } from "naive-ui";
import { AlertCircleOutline, SearchOutline } from "@vicons/ionicons5";
import { mediaURL } from "../../api/client";
import { tileLetter, useAppStore } from "../app/composables/useApp";

const app = useAppStore();
</script>

<template>
  <section class="panel library">
    <div class="lib-toolbar">
      <n-select
        v-model:value="app.platformFilter"
        :options="app.platformOptions"
        class="platform-select"
        @update:value="app.refreshGames"
      />
      <n-input
        v-model:value="app.query"
        class="search-input"
        :placeholder="app.t('app.search')"
        clearable
        @keyup.enter="app.refreshGames"
        @clear="app.refreshGames"
      >
        <template #prefix>
          <n-icon :component="SearchOutline" :size="16" />
        </template>
      </n-input>
      <n-button secondary class="search-btn" @click="app.refreshGames">
        <template #icon>
          <n-icon :component="SearchOutline" />
        </template>
        {{ app.t("app.searchAction") }}
      </n-button>
    </div>
    <div v-if="!app.games.length" class="empty">{{ app.t("library.empty") }}</div>
    <div v-else class="cover-grid">
      <button v-for="g in app.games" :key="g.id" class="cover" @click="app.openGame(g)">
        <div class="art" :data-tone="g.platform_id.length % 5">
          <img v-if="g.cover_path" :src="mediaURL(g.cover_path)" alt="" loading="lazy" />
          <span v-else>{{ tileLetter(g.title_zh || g.filename) }}</span>
        </div>
        <div class="meta">
          <strong>{{ g.title_zh || g.filename }}</strong>
          <small>{{ g.platform_name }}</small>
        </div>
        <div class="badges">
          <n-tag v-if="g.missing" size="tiny" type="warning" round>
            <template #icon>
              <n-icon :component="AlertCircleOutline" />
            </template>
            {{ app.t("library.missing") }}
          </n-tag>
        </div>
      </button>
    </div>
  </section>
</template>

<style scoped>
.lib-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
  margin-bottom: 1rem;
  align-items: center;
}

.search-input {
  flex: 1 1 160px;
  min-width: 0;
}

.search-btn {
  flex: 0 0 auto;
}

.cover-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
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
  border-radius: 0.55rem;
  overflow: hidden;
  background: hsl(var(--card) / 0.65);
  border: 1px solid hsl(var(--border) / 0.55);
  transition:
    transform 160ms ease,
    box-shadow 160ms ease;
}

.cover:hover {
  transform: translateY(-3px);
  box-shadow: 0 16px 40px hsl(var(--foreground) / 0.12);
}

.meta {
  padding: 0.7rem 0.75rem 0.85rem;
  min-width: 0;
}

.meta strong {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  font-size: 0.92rem;
  line-height: 1.25;
  word-break: break-word;
}

.meta small {
  display: block;
  margin-top: 0.2rem;
  opacity: 0.6;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.badges {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.25rem;
  max-width: calc(100% - 1rem);
}

@media (min-width: 720px) {
  .cover-grid {
    grid-template-columns: repeat(auto-fill, minmax(132px, 1fr));
    gap: 0.85rem;
  }
}
</style>
