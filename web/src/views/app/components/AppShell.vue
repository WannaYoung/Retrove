<script setup lang="ts">
import { NIcon } from "naive-ui";
import { LogOutOutline } from "@vicons/ionicons5";
import { useAppStore } from "../composables/useApp";
import GameDrawer from "../../library/components/GameDrawer.vue";

const app = useAppStore();
</script>

<template>
  <div class="shell" :class="{ 'has-drawer': !!app.selected }">
    <aside class="rail">
      <div class="rail-brand">
        <span class="brand-mark sm">R</span>
        <div class="rail-brand-text">
          <strong>{{ app.t("app.name") }}</strong>
          <small>{{ app.me?.username }}</small>
        </div>
      </div>
      <nav class="rail-nav">
        <router-link
          v-for="item in app.navItems"
          :key="item.id"
          :to="item.to"
          class="nav-link"
          :class="{ active: app.view === item.id }"
        >
          <n-icon :component="item.icon" :size="18" />
          <span>{{ app.t(item.labelKey) }}</span>
        </router-link>
      </nav>
      <button class="logout" @click="app.logout">
        <n-icon :component="LogOutOutline" :size="18" />
        <span>{{ app.t("app.logout") }}</span>
      </button>
    </aside>

    <main class="stage">
      <header class="topbar mobile-only-bar">
        <div class="mobile-brand">
          <span class="brand-mark sm">R</span>
          <strong>{{ app.t("app.name") }}</strong>
        </div>
        <button class="logout mobile-only" @click="app.logout">
          <n-icon :component="LogOutOutline" :size="16" />
          <span>{{ app.t("app.logout") }}</span>
        </button>
      </header>

      <router-view />
    </main>

    <GameDrawer />

    <nav class="bottom-nav" aria-label="main">
      <router-link
        v-for="item in app.navItems"
        :key="item.id"
        :to="item.to"
        class="nav-link"
        :class="{ active: app.view === item.id }"
      >
        <n-icon :component="item.icon" :size="20" />
        <span>{{ app.t(item.labelKey) }}</span>
      </router-link>
    </nav>
  </div>
</template>

<style scoped>
.shell {
  display: grid;
  grid-template-columns: 1fr;
  grid-template-rows: 1fr;
  height: 100dvh;
  min-height: 100vh;
  min-height: 100dvh;
  overflow: hidden;
  font-family: "Outfit", "Noto Sans SC", system-ui, sans-serif;
  background:
    radial-gradient(ellipse 60% 40% at 100% 0%, hsl(var(--primary) / 0.12), transparent 55%),
    hsl(var(--background));
  color: hsl(var(--foreground));
}

.rail {
  display: none;
}

.bottom-nav {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 30;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 2px;
  padding: 6px 4px calc(6px + env(safe-area-inset-bottom));
  background: hsl(var(--card) / 0.95);
  border-top: 1px solid hsl(var(--border) / 0.8);
  backdrop-filter: blur(14px);
}

.bottom-nav .nav-link,
.rail-nav .nav-link,
.logout {
  border: 0;
  cursor: pointer;
  font: inherit;
  background: transparent;
  color: inherit;
  text-decoration: none;
}

.bottom-nav .nav-link {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  padding: 6px 2px;
  border-radius: 10px;
  font-size: 0.68rem;
  line-height: 1.15;
  opacity: 0.55;
  min-width: 0;
}

.bottom-nav .nav-link span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}

.bottom-nav .nav-link.active {
  opacity: 1;
  font-weight: 600;
  background: hsl(var(--muted) / 0.55);
  color: hsl(var(--primary));
}

.mobile-only-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.mobile-brand {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.mobile-only.logout {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: hsl(var(--foreground));
  padding: 0.35rem 0.7rem;
}

.rail-brand {
  display: flex;
  gap: 0.75rem;
  align-items: center;
  padding: 0.35rem 0.45rem 1rem;
}

.rail-brand-text {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.rail-brand-text strong,
.rail-brand-text small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rail-brand strong {
  margin: 0;
  letter-spacing: 0.02em;
}

.rail-brand small {
  margin: 0.2rem 0 0;
  opacity: 0.72;
}

.rail-nav {
  display: grid;
  gap: 0.35rem;
  flex: 1;
  align-content: start;
}

.rail-nav .nav-link,
.logout {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  text-align: left;
  padding: 0.75rem 0.9rem;
  border-radius: 0.9rem;
  color: hsl(var(--sidebar-foreground));
}

.rail-nav .nav-link.active,
.rail-nav .nav-link:hover {
  background: hsl(var(--sidebar-accent) / 0.9);
}

.rail-nav .nav-link.active {
  color: hsl(var(--primary));
  font-weight: 600;
}

.logout {
  opacity: 0.75;
}

.stage {
  min-width: 0;
  min-height: 0;
  overflow: auto;
  padding: 1rem 1rem 6.5rem;
}

@media (min-width: 720px) {
  .stage {
    padding: 1.25rem 1.5rem 6.5rem;
  }
}

@media (min-width: 1024px) {
  .shell {
    grid-template-columns: 220px minmax(0, 1fr);
  }

  .shell.has-drawer {
    grid-template-columns: 220px minmax(0, 1fr) minmax(300px, 380px);
  }

  .rail {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    min-height: 0;
    height: 100%;
    overflow: auto;
    padding: 1.2rem 0.9rem;
    border-right: 1px solid hsl(var(--sidebar-border) / 0.8);
    background: hsl(var(--sidebar-background) / 0.85);
    backdrop-filter: blur(16px);
  }

  .bottom-nav,
  .mobile-only-bar,
  .mobile-only {
    display: none;
  }

  .stage {
    padding: 1.25rem 1.5rem 2rem;
  }
}
</style>
