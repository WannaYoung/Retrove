<script setup lang="ts">
import { NButton, NForm, NFormItem, NInput } from "naive-ui";
import { useAppStore } from "../app/composables/useApp";

const app = useAppStore();
</script>

<template>
  <div class="auth-shell">
    <div class="auth-panel">
      <div class="brand-block">
        <div class="brand-mark">R</div>
        <div>
          <h1>{{ app.t("app.name") }}</h1>
          <p>{{ app.t("app.tagline") }}</p>
        </div>
      </div>
      <h2>{{ app.hasUser ? app.t("auth.welcome") : app.t("auth.setup") }}</h2>
      <n-form class="auth-form" @submit.prevent="app.submitAuth">
        <n-form-item :label="app.t('auth.username')">
          <n-input v-model:value="app.username" :placeholder="app.t('auth.username')" size="large" />
        </n-form-item>
        <n-form-item v-if="!app.hasUser" :label="app.t('auth.email')">
          <n-input v-model:value="app.email" size="large" />
        </n-form-item>
        <n-form-item :label="app.t('auth.password')">
          <n-input
            v-model:value="app.password"
            type="password"
            show-password-on="click"
            :placeholder="app.t('auth.passwordHint')"
            size="large"
          />
        </n-form-item>
        <n-form-item v-if="!app.hasUser" :label="app.t('auth.passwordConfirm')">
          <n-input
            v-model:value="app.passwordConfirm"
            type="password"
            show-password-on="click"
            :placeholder="app.t('auth.passwordConfirmHint')"
            size="large"
          />
        </n-form-item>
        <p v-if="app.error" class="err">{{ app.error }}</p>
        <n-button attr-type="submit" type="primary" block size="large" :loading="app.submitting">
          {{ app.hasUser ? app.t("auth.login") : app.t("auth.register") }}
        </n-button>
      </n-form>
    </div>
  </div>
</template>

<style scoped>
.auth-shell {
  position: relative;
  display: grid;
  place-items: center;
  min-height: 100vh;
  min-height: 100dvh;
  overflow: hidden;
  font-family: "Outfit", "Noto Sans SC", system-ui, sans-serif;
  background:
    radial-gradient(ellipse 80% 60% at 20% 20%, hsl(var(--primary) / 0.22), transparent 55%),
    radial-gradient(ellipse 70% 50% at 90% 80%, hsl(var(--accent) / 0.18), transparent 50%),
    hsl(var(--background));
  color: hsl(var(--foreground));
}

.auth-panel {
  width: min(420px, calc(100vw - 2rem));
  padding: 2rem;
  border-radius: 1.5rem;
  background: hsl(var(--card) / 0.72);
  border: 1px solid hsl(var(--border) / 0.7);
  backdrop-filter: blur(18px);
  box-shadow: 0 24px 80px hsl(var(--foreground) / 0.12);
}

.brand-block {
  display: flex;
  gap: 0.9rem;
  align-items: center;
  margin-bottom: 1.5rem;
}

.brand-block h1 {
  margin: 0;
  letter-spacing: 0.02em;
}

.brand-block p {
  margin: 0.2rem 0 0;
  opacity: 0.72;
}

.auth-form :deep(.n-form-item) {
  margin-bottom: 0.85rem;
}
</style>
