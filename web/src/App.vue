<script setup lang="ts">
import { provide } from "vue";
import { NConfigProvider } from "naive-ui";
import { APP_KEY, provideApp } from "./views/app/composables/useApp";
import "./views/app/ui.css";

const app = provideApp();
provide(APP_KEY, app);
</script>

<template>
  <n-config-provider
    :theme="app.naiveTheme"
    :locale="app.naiveLocale"
    :date-locale="app.naiveDateLocale"
    :theme-overrides="app.naiveOverrides"
  >
    <div v-if="!app.ready" class="boot">{{ app.t("app.loading") }}</div>
    <router-view v-else />
  </n-config-provider>
</template>

<style scoped>
.boot {
  display: grid;
  place-items: center;
  min-height: 100vh;
  min-height: 100dvh;
  font-family: "Outfit", "Noto Sans SC", system-ui, sans-serif;
  background: hsl(var(--background));
  color: hsl(var(--foreground));
}
</style>
