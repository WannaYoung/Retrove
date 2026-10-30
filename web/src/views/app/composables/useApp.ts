import { computed, inject, onMounted, reactive, ref, watch, type InjectionKey } from "vue";
import { useI18n } from "vue-i18n";
import { zhCN, enUS, dateZhCN, dateEnUS } from "naive-ui";
import {
  AlbumsOutline,
  ColorWandOutline,
  HomeOutline,
  SettingsOutline,
} from "@vicons/ionicons5";
import {
  api,
  waitJob,
  type Game,
  type Job,
  type MediaKind,
  type Platform,
  type ScrapeFailure,
  type ScrapeFields,
  type ScrapeReport,
  type ScrapeRequest,
  type ScrapeStatus,
  type ScrapeWriteMode,
  type Settings,
  type Stats,
  type User,
} from "../../../api/client";
import { usePreferences, type LocaleId } from "../../../preferences/usePreferences";
import type { ThemeId } from "../../../preferences/themes";
import { isSettingsTab, router, type SettingsTab } from "../../../router";

export type View = "home" | "library" | "scrape" | "settings";
export type ProviderId = "ss" | "igdb" | "sgdb";
export type { SettingsTab };

export function tileLetter(title: string) {
  const s = (title || "?").trim();
  return s.slice(0, 1).toUpperCase();
}

function createApp() {
  const { t, locale } = useI18n();
  const { themeId, localeId, themeOptions, naiveTheme, naiveOverrides, setTheme, setLocale, initPreferences } =
    usePreferences();

  const ready = ref(false);
  const hasUser = ref(false);
  const me = ref<User | null>(null);
  const error = ref("");
  const submitting = ref(false);
  const busy = ref(false);
  const busyLabel = ref("");

  const username = ref("");
  const email = ref("");
  const password = ref("");
  const passwordConfirm = ref("");

  const stats = ref<Stats | null>(null);
  const platforms = ref<Platform[]>([]);
  const games = ref<Game[]>([]);
  const recentGames = ref<Game[]>([]);
  const jobs = ref<Job[]>([]);
  const settings = ref<Settings | null>(null);
  const scrapeStatus = ref<ScrapeStatus | null>(null);
  const platformFilter = ref("");
  const scrapePlatformId = ref("");
  const query = ref("");
  const selected = ref<Game | null>(null);
  const editTitleZh = ref("");
  const editTitleEn = ref("");
  const editDeveloper = ref("");
  const editDescription = ref("");
  const editReleaseYear = ref<number | null>(null);
  const manualScrapeOpen = ref(false);
  const mediaUploading = ref<string | null>(null);
  const lastReport = ref<unknown>(null);
  const scrapeReport = ref<ScrapeReport | null>(null);
  const scrapeFailures = ref<ScrapeFailure[]>([]);
  const jobProgress = ref(0);
  const ssDevId = ref("");
  const ssDevPassword = ref("");
  const ssUser = ref("");
  const ssUserPassword = ref("");
  const ssSoftname = ref("Retrove");
  const igdbClientId = ref("");
  const igdbClientSecret = ref("");
  const sgdbApiKey = ref("");
  const providerModal = ref<ProviderId | null>(null);
  const providerSaving = ref(false);
  const providerSaved = ref(false);
  const providerTesting = ref<ProviderId | null>(null);
  const providerTestResult = ref<{ id: ProviderId; ok: boolean; message: string } | null>(null);
  const llmBaseUrl = ref("");
  const llmApiKey = ref("");
  const llmModel = ref("");
  const llmModels = ref<{ label: string; value: string }[]>([]);
  const llmSaving = ref(false);
  const llmSaved = ref(false);
  const llmTesting = ref(false);
  const llmFetchingModels = ref(false);
  const llmTestResult = ref<{ ok: boolean; message: string } | null>(null);
  const translatePlatformId = ref<string | null>(null);

  const providerApiId: Record<ProviderId, string> = {
    ss: "screenscraper",
    igdb: "igdb",
    sgdb: "steamgriddb",
  };

  const anyScrapeConfigured = computed(
    () =>
      !!(
        scrapeStatus.value?.any_configured ||
        settings.value?.any_scrape_configured ||
        scrapeStatus.value?.screenscraper_configured ||
        scrapeStatus.value?.igdb_configured ||
        scrapeStatus.value?.steamgriddb_configured
      ),
  );

  const naiveLocale = computed(() => (localeId.value === "zh-CN" ? zhCN : enUS));
  const naiveDateLocale = computed(() => (localeId.value === "zh-CN" ? dateZhCN : dateEnUS));
  const languageOptions = [
    { label: "中文", value: "zh-CN" },
    { label: "English", value: "en-US" },
  ];
  const themeSelectOptions = computed(() =>
    themeOptions.value.map((item) => ({ label: t(item.nameKey), value: item.id })),
  );
  const platformOptions = computed(() => [
    { label: t("app.allPlatforms"), value: "" },
    ...platforms.value.map((p) => ({ label: p.name, value: p.id })),
  ]);
  const romPlatforms = computed(() => platforms.value.filter((p) => (p.rom_count ?? 0) > 0));
  const scrapePlatformOptions = computed(() =>
    romPlatforms.value.map((p) => ({
      label: `${p.name} (${p.rom_count ?? 0})`,
      value: p.id,
    })),
  );
  const llmConfigured = computed(
    () =>
      !!(
        settings.value?.llm_configured ||
        settings.value?.providers?.llm?.configured ||
        (llmBaseUrl.value.trim() && llmModel.value.trim() && (llmApiKey.value.trim() || settings.value?.providers?.llm?.api_key_set))
      ),
  );

  const view = computed<View>(() => {
    const name = router.currentRoute.value.name;
    if (name === "game") return "library";
    if (name === "library" || name === "scrape" || name === "settings" || name === "home") return name;
    return "home";
  });

  const settingsTab = computed<SettingsTab>({
    get() {
      const tab = router.currentRoute.value.params.tab;
      return isSettingsTab(tab) ? tab : "appearance";
    },
    set(tab) {
      void router.push({ name: "settings", params: { tab } });
    },
  });

  const navItems: { id: View; to: string; icon: typeof HomeOutline; labelKey: string }[] = [
    { id: "home", to: "/", icon: HomeOutline, labelKey: "nav.home" },
    { id: "library", to: "/library", icon: AlbumsOutline, labelKey: "nav.library" },
    { id: "scrape", to: "/scrape", icon: ColorWandOutline, labelKey: "nav.scrape" },
    { id: "settings", to: "/settings", icon: SettingsOutline, labelKey: "nav.settings" },
  ];

  watch(localeId, (id) => {
    locale.value = id;
  });

  function syncAuthRoute() {
    if (!ready.value) return;
    const name = router.currentRoute.value.name;
    if (!me.value && name !== "login") {
      void router.replace({ name: "login", query: { redirect: router.currentRoute.value.fullPath } });
      return;
    }
    if (me.value && name === "login") {
      const redirect = router.currentRoute.value.query.redirect;
      void router.replace(typeof redirect === "string" && redirect.startsWith("/") ? redirect : { name: "home" });
    }
  }

  watch([ready, me], () => syncAuthRoute());

  watch(
    () => ({
      authed: !!me.value,
      name: router.currentRoute.value.name,
      id: router.currentRoute.value.params.id,
    }),
    ({ authed, name, id }) => {
      if (!authed) return;
      if (name === "library" || name === "game") refreshGames().catch(() => undefined);
      if (name === "home") refreshHome().catch(() => undefined);
      if (name === "settings") {
        api.settings()
          .then((s) => {
            settings.value = s;
            hydrateProviderForms(s);
          })
          .catch(() => undefined);
        api.jobs()
          .then((r) => (jobs.value = r.items))
          .catch(() => undefined);
      }
      if (name === "scrape") {
        void refreshScrape().catch(() => undefined);
      }
      if (name === "game") {
        const gameId = Number(id);
        if (!Number.isFinite(gameId) || gameId <= 0) {
          void router.replace({ name: "library" });
          return;
        }
        api
          .game(gameId)
          .then((g) => applySelectedGame(g))
          .catch(() => {
            selected.value = null;
            void router.replace({ name: "library" });
          });
        return;
      }
      if (selected.value) selected.value = null;
    },
    { immediate: true },
  );

  function hydrateProviderForms(se: Settings) {
    const ss = se.providers?.screenscraper;
    const ig = se.providers?.igdb;
    const llm = se.providers?.llm;
    ssDevId.value = ss?.dev_id || "";
    ssUser.value = ss?.user_ss || "";
    ssSoftname.value = ss?.softname || "Retrove";
    igdbClientId.value = ig?.client_id || "";
    llmBaseUrl.value = llm?.base_url || "";
    llmModel.value = llm?.model || "";
    if (llm?.model && !llmModels.value.some((m) => m.value === llm.model)) {
      llmModels.value = [{ label: llm.model, value: llm.model }, ...llmModels.value];
    }
  }

  async function bootstrap() {
    const status = await api.authStatus();
    hasUser.value = status.has_user;
    me.value = await api.me();
    ready.value = true;
    if (me.value) await refreshAll();
  }

  async function refreshHome() {
    const [st, pl, jb, sc, recent] = await Promise.all([
      api.stats(),
      api.platforms(),
      api.jobs(),
      api.scrapeStatus().catch(() => null),
      api.games({ limit: 12, sort: "updated" }).catch(() => ({ items: [] as Game[] })),
    ]);
    stats.value = st;
    platforms.value = pl.items;
    jobs.value = jb.items;
    scrapeStatus.value = sc;
    recentGames.value = recent.items;
    const withRoms = platforms.value.filter((p) => (p.rom_count ?? 0) > 0);
    if (!scrapePlatformId.value || !withRoms.some((p) => p.id === scrapePlatformId.value)) {
      scrapePlatformId.value = withRoms[0]?.id || "";
    }
  }

  async function refreshScrape() {
    const [st, pl, sc] = await Promise.all([
      api.stats(),
      api.platforms(),
      api.scrapeStatus().catch(() => null),
    ]);
    stats.value = st;
    platforms.value = pl.items;
    scrapeStatus.value = sc;
    const withRoms = platforms.value.filter((p) => (p.rom_count ?? 0) > 0);
    if (!scrapePlatformId.value || !withRoms.some((p) => p.id === scrapePlatformId.value)) {
      scrapePlatformId.value = withRoms[0]?.id || "";
    }
  }

  async function refreshAll() {
    const [st, pl, gm, jb, se, sc, recent] = await Promise.all([
      api.stats(),
      api.platforms(),
      api.games({ platform: platformFilter.value || undefined, q: query.value || undefined, limit: 300 }),
      api.jobs(),
      api.settings(),
      api.scrapeStatus().catch(() => null),
      api.games({ limit: 12, sort: "updated" }).catch(() => ({ items: [] as Game[] })),
    ]);
    stats.value = st;
    platforms.value = pl.items;
    games.value = gm.items;
    jobs.value = jb.items;
    settings.value = se;
    scrapeStatus.value = sc;
    recentGames.value = recent.items;
    hydrateProviderForms(se);
    const withRoms = platforms.value.filter((p) => (p.rom_count ?? 0) > 0);
    if (!scrapePlatformId.value || !withRoms.some((p) => p.id === scrapePlatformId.value)) {
      scrapePlatformId.value = withRoms[0]?.id || "";
    }
  }

  async function refreshGames() {
    const gm = await api.games({
      platform: platformFilter.value || undefined,
      q: query.value || undefined,
      limit: 300,
    });
    games.value = gm.items;
  }

  onMounted(() => {
    initPreferences();
    locale.value = localeId.value;
    bootstrap().catch(() => {
      error.value = t("app.backendDown");
      ready.value = true;
    });
  });

  async function submitAuth() {
    error.value = "";
    if (!hasUser.value) {
      if (!username.value.trim() || !email.value.trim()) {
        error.value = t("auth.needUsernameEmail");
        return;
      }
      if (password.value.length < 8) {
        error.value = t("auth.passwordTooShort");
        return;
      }
      if (password.value !== passwordConfirm.value) {
        error.value = t("auth.passwordMismatch");
        return;
      }
    }
    submitting.value = true;
    try {
      if (hasUser.value) {
        await api.login({ username: username.value, password: password.value });
      } else {
        await api.register({
          username: username.value,
          email: email.value,
          password: password.value,
          password_confirm: passwordConfirm.value,
        });
      }
      username.value = password.value = passwordConfirm.value = email.value = "";
      await bootstrap();
    } catch (e) {
      error.value = e instanceof Error ? e.message : t("auth.submitFailed");
    } finally {
      submitting.value = false;
    }
  }

  async function logout() {
    await api.logout();
    me.value = null;
    await bootstrap();
    void router.replace({ name: "login" });
  }

  async function runJob(kind: "scan" | "import") {
    busy.value = true;
    busyLabel.value = t("scan.running");
    jobProgress.value = 0;
    go("settings", "tasks");
    try {
      const job = kind === "scan" ? await api.scan() : await api.importPegasus();
      const done = await waitJob(job.id, (j) => {
        jobProgress.value = j.progress;
        busyLabel.value = j.message || t("scan.running");
      });
      lastReport.value = done.report ?? { message: done.message, status: done.status };
      await refreshAll();
      go("settings", "tasks");
    } catch (e) {
      lastReport.value = { error: e instanceof Error ? e.message : "failed" };
      go("settings", "tasks");
    } finally {
      busy.value = false;
      busyLabel.value = "";
    }
  }

  async function saveLLM() {
    llmSaving.value = true;
    llmSaved.value = false;
    llmTestResult.value = null;
    try {
      const body: Record<string, string> = {
        base_url: llmBaseUrl.value.trim(),
        model: llmModel.value.trim(),
      };
      if (llmApiKey.value.trim()) body.api_key = llmApiKey.value.trim();
      await api.updateLLM(body);
      llmApiKey.value = "";
      llmSaved.value = true;
      settings.value = await api.settings();
      hydrateProviderForms(settings.value);
    } catch (e) {
      error.value = e instanceof Error ? e.message : t("auth.submitFailed");
    } finally {
      llmSaving.value = false;
    }
  }

  async function fetchLLMModels() {
    llmFetchingModels.value = true;
    llmTestResult.value = null;
    try {
      const params: { base_url?: string; api_key?: string } = {};
      if (llmBaseUrl.value.trim()) params.base_url = llmBaseUrl.value.trim();
      if (llmApiKey.value.trim()) params.api_key = llmApiKey.value.trim();
      const res = await api.llmModels(params);
      llmModels.value = res.items.map((m) => ({ label: m.id, value: m.id }));
      if (llmModel.value && !llmModels.value.some((m) => m.value === llmModel.value)) {
        llmModels.value = [{ label: llmModel.value, value: llmModel.value }, ...llmModels.value];
      }
      if (!llmModel.value && llmModels.value[0]) {
        llmModel.value = llmModels.value[0].value;
      }
    } catch (e) {
      llmTestResult.value = {
        ok: false,
        message: e instanceof Error ? e.message : "failed",
      };
    } finally {
      llmFetchingModels.value = false;
    }
  }

  async function testLLM() {
    llmTesting.value = true;
    llmTestResult.value = null;
    try {
      const body: Record<string, string> = {};
      if (llmBaseUrl.value.trim()) body.base_url = llmBaseUrl.value.trim();
      if (llmApiKey.value.trim()) body.api_key = llmApiKey.value.trim();
      if (llmModel.value.trim()) body.model = llmModel.value.trim();
      const res = await api.testLLM(body);
      llmTestResult.value = { ok: res.ok, message: res.message || (res.ok ? "ok" : "failed") };
    } catch (e) {
      llmTestResult.value = {
        ok: false,
        message: e instanceof Error ? e.message : "failed",
      };
    } finally {
      llmTesting.value = false;
    }
  }

  async function runTranslatePlatform(platformId: string) {
    if (!platformId) return;
    busy.value = true;
    translatePlatformId.value = platformId;
    busyLabel.value = t("settings.llmRunning");
    jobProgress.value = 0;
    try {
      const job = await api.translatePlatform(platformId);
      const done = await waitJob(job.id, (j) => {
        jobProgress.value = j.progress;
        busyLabel.value = j.message || t("settings.llmRunning");
      });
      lastReport.value = done.report ?? { message: done.message, status: done.status };
      if (done.status === "failed") {
        lastReport.value = { error: done.message, report: done.report };
      }
      await refreshAll();
      go("settings", "translate");
    } catch (e) {
      lastReport.value = { error: e instanceof Error ? e.message : "failed" };
      go("settings", "translate");
    } finally {
      busy.value = false;
      busyLabel.value = "";
      translatePlatformId.value = null;
    }
  }

  async function runScrape(
    kind: "missing" | "platform" | "one" | "batch" = "batch",
    gameId?: number,
    opts?: {
      only_missing?: boolean;
      write_mode?: ScrapeWriteMode;
      fields?: Partial<ScrapeFields>;
      platform_id?: string;
    },
  ) {
    busy.value = true;
    busyLabel.value = t("scrape.running");
    jobProgress.value = 0;
    try {
      let job: Job;
      if (kind === "one") {
        job = await api.scrapeOne(gameId ?? selected.value?.id ?? 0, opts?.fields, opts?.write_mode);
      } else if (kind === "missing") {
        job = await api.scrapeMissing();
      } else if (kind === "platform") {
        job = await api.scrapePlatform(opts?.platform_id || scrapePlatformId.value, {
          only_missing: opts?.only_missing,
          write_mode: opts?.write_mode,
          fields: opts?.fields,
        });
      } else {
        const body: ScrapeRequest = {
          platform_id: opts?.platform_id || scrapePlatformId.value,
          only_missing: opts?.only_missing ?? false,
          write_mode: opts?.write_mode ?? "update",
          fields: opts?.fields,
        };
        job = await api.scrape(body);
      }
      const done = await waitJob(job.id, (j) => {
        jobProgress.value = j.progress;
        busyLabel.value = j.message || t("scrape.running");
      });
      const report = (done.report || null) as ScrapeReport | null;
      scrapeReport.value = report;
      scrapeFailures.value = report?.failed ?? [];
      lastReport.value = report ?? { message: done.message, status: done.status };
      if (done.status === "failed") {
        lastReport.value = { error: done.message, report };
      }
      await refreshAll();
      if (selected.value) {
        const g = await api.game(selected.value.id);
        applySelectedGame(g);
      }
      if (kind !== "one") go("scrape");
    } catch (e) {
      lastReport.value = { error: e instanceof Error ? e.message : "failed" };
      go("scrape");
    } finally {
      busy.value = false;
      busyLabel.value = "";
    }
  }

  async function retryScrapeFailures() {
    const ids = [...new Set(scrapeFailures.value.map((f) => f.game_id).filter(Boolean))];
    if (!ids.length) return;
    busy.value = true;
    busyLabel.value = t("scrape.running");
    jobProgress.value = 0;
    const remaining: ScrapeFailure[] = [];
    try {
      for (let i = 0; i < ids.length; i++) {
        const id = ids[i];
        jobProgress.value = i / ids.length;
        busyLabel.value = `#${id}`;
        try {
          const job = await api.scrapeOne(id);
          const done = await waitJob(job.id, (j) => {
            busyLabel.value = j.message || `#${id}`;
          });
          if (done.status === "failed") {
            remaining.push({ game_id: id, filename: String(id), kind: "failed", message: done.message });
          } else {
            const report = done.report as ScrapeReport | undefined;
            if (report?.failed?.length) remaining.push(...report.failed);
          }
        } catch (e) {
          remaining.push({
            game_id: id,
            filename: String(id),
            kind: "error",
            message: e instanceof Error ? e.message : "failed",
          });
        }
      }
      scrapeFailures.value = remaining;
      await refreshAll();
    } finally {
      busy.value = false;
      busyLabel.value = "";
      jobProgress.value = 1;
    }
  }

  function openProvider(id: ProviderId) {
    providerSaved.value = false;
    providerModal.value = id;
  }

  async function testProvider(id: ProviderId) {
    providerTesting.value = id;
    providerTestResult.value = null;
    try {
      const res = await api.testScrapeProvider(providerApiId[id]);
      providerTestResult.value = { id, ok: res.ok, message: res.message || (res.ok ? "ok" : "failed") };
    } catch (e) {
      providerTestResult.value = {
        id,
        ok: false,
        message: e instanceof Error ? e.message : "failed",
      };
    } finally {
      providerTesting.value = null;
    }
  }

  async function saveProvider() {
    if (!providerModal.value) return;
    providerSaving.value = true;
    providerSaved.value = false;
    try {
      if (providerModal.value === "ss") {
        const body: Record<string, string> = { softname: ssSoftname.value || "Retrove" };
        body.dev_id = ssDevId.value.trim();
        if (ssDevPassword.value) body.dev_password = ssDevPassword.value;
        body.user_ss = ssUser.value.trim();
        if (ssUserPassword.value) body.user_password = ssUserPassword.value;
        await api.updateScreenScraper(body);
        ssDevPassword.value = "";
        ssUserPassword.value = "";
      } else if (providerModal.value === "igdb") {
        const body: Record<string, string> = { client_id: igdbClientId.value.trim() };
        if (igdbClientSecret.value) body.client_secret = igdbClientSecret.value;
        await api.updateIGDB(body);
        igdbClientSecret.value = "";
      } else {
        const body: Record<string, string> = {};
        if (sgdbApiKey.value) body.api_key = sgdbApiKey.value;
        await api.updateSteamGridDB(body);
        sgdbApiKey.value = "";
      }
      settings.value = await api.settings();
      hydrateProviderForms(settings.value);
      scrapeStatus.value = await api.scrapeStatus();
      providerModal.value = null;
      providerSaved.value = false;
    } catch (e) {
      error.value = e instanceof Error ? e.message : t("auth.submitFailed");
    } finally {
      providerSaving.value = false;
    }
  }

  function applySelectedGame(g: Game) {
    selected.value = g;
    editTitleZh.value = g.title_zh;
    editTitleEn.value = g.title_en;
    editDeveloper.value = g.developer;
    editDescription.value = g.description;
    editReleaseYear.value = g.release_year > 0 ? g.release_year : null;
  }

  function mediaPath(kind: string) {
    const hit = selected.value?.media?.find((m) => m.kind === kind);
    return hit?.path || (kind === "box" ? selected.value?.cover_path || "" : "");
  }

  async function uploadMedia(kind: MediaKind, file: File) {
    if (!selected.value) return;
    mediaUploading.value = kind;
    try {
      const res = await api.uploadGameMedia(selected.value.id, kind, file);
      applySelectedGame(res.game);
      await refreshGames();
    } finally {
      mediaUploading.value = null;
    }
  }

  function openManualScrape() {
    void api.scrapeStatus()
      .then((s) => {
        scrapeStatus.value = s;
      })
      .catch(() => undefined);
    manualScrapeOpen.value = true;
  }

  function closeManualScrape() {
    manualScrapeOpen.value = false;
  }

  async function onManualScrapeSaved(g: Game) {
    applySelectedGame(g);
    manualScrapeOpen.value = false;
    await refreshGames();
  }

  function openGame(g: Game) {
    applySelectedGame(g);
    const route = router.currentRoute.value;
    if (route.name !== "game" || String(route.params.id) !== String(g.id)) {
      void router.push({ name: "game", params: { id: String(g.id) } });
    }
  }

  async function openGameById(id: number) {
    if (!id) return;
    try {
      const g = await api.game(id);
      openGame(g);
    } catch {
      void router.push({ name: "game", params: { id: String(id) } });
    }
  }

  function closeGame() {
    selected.value = null;
    if (router.currentRoute.value.name === "game") {
      void router.push({ name: "library" });
    }
  }

  async function saveGame() {
    if (!selected.value) return;
    const g = await api.updateGame(selected.value.id, {
      title_zh: editTitleZh.value,
      title_en: editTitleEn.value,
      developer: editDeveloper.value,
      description: editDescription.value,
      release_year: editReleaseYear.value && editReleaseYear.value > 0 ? editReleaseYear.value : 0,
    });
    applySelectedGame(g);
    await refreshGames();
  }

  function go(v: View, tab?: SettingsTab) {
    if (v === "settings") {
      void router.push({ name: "settings", params: { tab: tab || settingsTab.value || "appearance" } });
      return;
    }
    void router.push({ name: v });
  }

  function openLibrary(platformId?: string) {
    platformFilter.value = platformId ?? "";
    query.value = "";
    void refreshGames();
    go("library");
  }

  return reactive({
    t,
    themeId,
    localeId,
    naiveTheme,
    naiveOverrides,
    naiveLocale,
    naiveDateLocale,
    setTheme,
    setLocale,
    languageOptions,
    themeSelectOptions,
    ready,
    hasUser,
    me,
    error,
    submitting,
    view,
    busy,
    busyLabel,
    username,
    email,
    password,
    passwordConfirm,
    stats,
    platforms,
    games,
    recentGames,
    jobs,
    settings,
    scrapeStatus,
    platformFilter,
    scrapePlatformId,
    query,
    selected,
    editTitleZh,
    editTitleEn,
    editDeveloper,
    editDescription,
    editReleaseYear,
    manualScrapeOpen,
    mediaUploading,
    lastReport,
    scrapeReport,
    scrapeFailures,
    jobProgress,
    ssDevId,
    ssDevPassword,
    ssUser,
    ssUserPassword,
    ssSoftname,
    igdbClientId,
    igdbClientSecret,
    sgdbApiKey,
    providerModal,
    providerSaving,
    providerSaved,
    providerTesting,
    providerTestResult,
    settingsTab,
    llmBaseUrl,
    llmApiKey,
    llmModel,
    llmModels,
    llmSaving,
    llmSaved,
    llmTesting,
    llmFetchingModels,
    llmTestResult,
    translatePlatformId,
    llmConfigured,
    romPlatforms,
    anyScrapeConfigured,
    platformOptions,
    scrapePlatformOptions,
    navItems,
    submitAuth,
    logout,
    runJob,
    runScrape,
    retryScrapeFailures,
    openProvider,
    testProvider,
    saveProvider,
    saveLLM,
    fetchLLMModels,
    testLLM,
    runTranslatePlatform,
    openGame,
    openGameById,
    closeGame,
    saveGame,
    mediaPath,
    uploadMedia,
    openManualScrape,
    closeManualScrape,
    onManualScrapeSaved,
    refreshGames,
    go,
    openLibrary,
  });
}

export type AppStore = ReturnType<typeof createApp>;
export const APP_KEY: InjectionKey<AppStore> = Symbol("retrove.app");

export function provideApp(): AppStore {
  const app = createApp();
  return app;
}

export function useAppStore(): AppStore {
  const app = inject(APP_KEY);
  if (!app) throw new Error("App store missing");
  return app;
}

export type { LocaleId, ThemeId };
