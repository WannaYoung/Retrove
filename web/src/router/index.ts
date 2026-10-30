import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";

export const SETTINGS_TABS = ["appearance", "tasks", "translate", "providers"] as const;
export type SettingsTab = (typeof SETTINGS_TABS)[number];

export function isSettingsTab(value: unknown): value is SettingsTab {
  return typeof value === "string" && (SETTINGS_TABS as readonly string[]).includes(value);
}

const routes: RouteRecordRaw[] = [
  {
    path: "/login",
    name: "login",
    component: () => import("../views/auth/AuthView.vue"),
    meta: { public: true },
  },
  {
    path: "/",
    component: () => import("../views/app/components/AppShell.vue"),
    meta: { requiresAuth: true },
    children: [
      { path: "", name: "home", component: () => import("../views/home/HomeView.vue") },
      { path: "library", name: "library", component: () => import("../views/library/LibraryView.vue") },
      {
        path: "library/:id(\\d+)",
        name: "game",
        component: () => import("../views/library/LibraryView.vue"),
      },
      { path: "scrape", name: "scrape", component: () => import("../views/scrape/ScrapeView.vue") },
      {
        path: "settings/:tab?",
        name: "settings",
        component: () => import("../views/settings/SettingsView.vue"),
        beforeEnter: (to) => {
          const tab = to.params.tab;
          if (tab && !isSettingsTab(tab)) {
            return { name: "settings", params: { tab: "appearance" }, replace: true };
          }
          return true;
        },
      },
    ],
  },
  { path: "/:pathMatch(.*)*", redirect: "/" },
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
});
