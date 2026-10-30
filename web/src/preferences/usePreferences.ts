import { computed, ref, watch } from "vue";
import { darkTheme, type GlobalTheme, type GlobalThemeOverrides } from "naive-ui";
import { DEFAULT_THEME, THEMES, isThemeId, type ThemeId } from "./themes";

const THEME_KEY = "retrove.theme";
const LOCALE_KEY = "retrove.locale";

export type LocaleId = "zh-CN" | "en-US";

function detectLocale(): LocaleId {
  const saved = localStorage.getItem(LOCALE_KEY);
  if (saved === "zh-CN" || saved === "en-US") return saved;
  const lang = navigator.language.toLowerCase();
  return lang.startsWith("zh") ? "zh-CN" : "en-US";
}

function detectTheme(): ThemeId {
  const saved = localStorage.getItem(THEME_KEY);
  if (saved && isThemeId(saved)) return saved;
  return DEFAULT_THEME;
}

function clearThemeClasses(el: HTMLElement) {
  el.classList.remove("homebox");
  for (const t of THEMES) {
    if (t.cssClass !== "homebox") el.classList.remove(t.cssClass);
  }
}

function applyThemeClass(id: ThemeId) {
  const el = document.documentElement;
  clearThemeClasses(el);
  const meta = THEMES.find((t) => t.id === id) ?? THEMES[0];
  el.classList.add(meta.cssClass);
}

/** Naive UI / seemly only accepts classic `hsl(h, s%, l%)`, not modern space syntax. */
function cssVar(name: string): string {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  if (!raw) return "#888888";
  const [hslPart, alpha] = raw.split("/").map((part) => part.trim());
  const [h, s, l] = hslPart.split(/\s+/);
  if (!h || !s || !l) return "#888888";
  if (alpha) return `hsla(${h}, ${s}, ${l}, ${alpha})`;
  return `hsl(${h}, ${s}, ${l})`;
}

function backgroundLightness(): number {
  const raw = getComputedStyle(document.documentElement).getPropertyValue("--background").trim();
  const l = parseFloat(raw.split(/\s+/)[2] || "50");
  return Number.isFinite(l) ? l : 50;
}

function isDarkSurface(): boolean {
  return backgroundLightness() < 42;
}

function buildNaiveOverrides(): GlobalThemeOverrides {
  const background = cssVar("--background");
  const foreground = cssVar("--foreground");
  const muted = cssVar("--muted");
  const mutedFg = cssVar("--muted-foreground");
  const card = cssVar("--card");
  const cardFg = cssVar("--card-foreground");
  const popover = cssVar("--popover");
  const popoverFg = cssVar("--popover-foreground");
  const primary = cssVar("--primary");
  const primaryFg = cssVar("--primary-foreground");
  const secondary = cssVar("--secondary");
  const accent = cssVar("--accent");
  const border = cssVar("--border");
  const input = cssVar("--input");
  const ring = cssVar("--ring");
  const destructive = cssVar("--destructive");
  const sidebar = cssVar("--sidebar-background");
  const sidebarFg = cssVar("--sidebar-foreground");

  return {
    common: {
      primaryColor: primary,
      primaryColorHover: primary,
      primaryColorPressed: primary,
      primaryColorSuppl: primary,
      infoColor: secondary,
      successColor: primary,
      warningColor: accent,
      errorColor: destructive,
      textColorBase: foreground,
      textColor1: foreground,
      textColor2: mutedFg,
      textColor3: mutedFg,
      textColorDisabled: mutedFg,
      bodyColor: background,
      cardColor: card,
      modalColor: popover,
      popoverColor: popover,
      tableColor: card,
      tableColorHover: muted,
      tableHeaderColor: muted,
      inputColor: input,
      inputColorDisabled: muted,
      actionColor: muted,
      hoverColor: muted,
      pressedColor: muted,
      borderColor: border,
      dividerColor: border,
      closeIconColor: mutedFg,
      closeIconColorHover: foreground,
      closeIconColorPressed: foreground,
      closeColorHover: muted,
      closeColorPressed: muted,
      clearColor: mutedFg,
      clearColorHover: foreground,
      clearColorPressed: foreground,
      scrollbarColor: muted,
      scrollbarColorHover: mutedFg,
      baseColor: card,
      invertedColor: foreground,
      tagColor: muted,
      tabColor: muted,
      avatarColor: muted,
      buttonColor2: muted,
      buttonColor2Hover: input,
      buttonColor2Pressed: border,
      opacityDisabled: "0.45",
      borderRadius: "var(--radius)",
    },
    Layout: {
      color: background,
      siderColor: sidebar,
      headerColor: background,
      textColor: foreground,
    },
    Card: {
      color: card,
      colorModal: popover,
      colorEmbedded: muted,
      textColor: cardFg,
      titleTextColor: foreground,
      borderColor: border,
      actionColor: muted,
    },
    Dialog: {
      color: popover,
      textColor: popoverFg,
      titleTextColor: foreground,
      closeIconColor: mutedFg,
      closeIconColorHover: foreground,
      closeColorHover: muted,
      iconColor: primary,
    },
    Modal: {
      color: popover,
      textColor: popoverFg,
    },
    Popover: {
      color: popover,
      textColor: popoverFg,
    },
    Form: {
      labelTextColor: foreground,
      feedbackTextColor: mutedFg,
      feedbackTextColorError: destructive,
    },
    Button: {
      textColor: foreground,
      textColorHover: primary,
      textColorPressed: primary,
      textColorFocus: primary,
      textColorText: foreground,
      textColorGhost: foreground,
      textColorPrimary: primaryFg,
      colorPrimary: primary,
      colorHoverPrimary: primary,
      colorPressedPrimary: primary,
      colorFocusPrimary: primary,
      border: `1px solid ${border}`,
      borderHover: `1px solid ${primary}`,
      borderPressed: `1px solid ${primary}`,
      borderFocus: `1px solid ${primary}`,
      borderPrimary: `1px solid ${primary}`,
      borderHoverPrimary: `1px solid ${primary}`,
      borderPressedPrimary: `1px solid ${primary}`,
      colorSecondary: muted,
      colorSecondaryHover: input,
      colorSecondaryPressed: border,
      textColorSecondary: foreground,
      textColorHoverSecondary: foreground,
      textColorPressedSecondary: foreground,
      borderSecondary: `1px solid ${border}`,
      borderHoverSecondary: `1px solid ${border}`,
      borderPressedSecondary: `1px solid ${border}`,
    },
    Input: {
      color: card,
      colorFocus: card,
      textColor: foreground,
      caretColor: primary,
      border: `1px solid ${border}`,
      borderHover: `1px solid ${ring}`,
      borderFocus: `1px solid ${ring}`,
      placeholderColor: mutedFg,
      iconColor: mutedFg,
      iconColorHover: foreground,
      clearColor: mutedFg,
      clearColorHover: foreground,
      suffixTextColor: mutedFg,
    },
    InternalSelection: {
      color: card,
      textColor: foreground,
      placeholderColor: mutedFg,
      border: `1px solid ${border}`,
      borderHover: `1px solid ${ring}`,
      borderActive: `1px solid ${ring}`,
      borderFocus: `1px solid ${ring}`,
      caretColor: primary,
      arrowColor: mutedFg,
      clearColor: mutedFg,
      clearColorHover: foreground,
    },
    InternalSelectMenu: {
      color: popover,
      optionTextColor: foreground,
      optionTextColorActive: primary,
      optionTextColorPressed: primary,
      optionColorPending: muted,
      optionColorActive: muted,
      optionColorActivePending: muted,
      groupHeaderTextColor: mutedFg,
      actionTextColor: foreground,
      actionDividerColor: border,
      loadingColor: primary,
    },
    Select: {
      peers: {
        InternalSelection: {
          color: card,
          textColor: foreground,
          placeholderColor: mutedFg,
          border: `1px solid ${border}`,
          borderHover: `1px solid ${ring}`,
          borderActive: `1px solid ${ring}`,
          borderFocus: `1px solid ${ring}`,
          caretColor: primary,
          arrowColor: mutedFg,
        },
        InternalSelectMenu: {
          color: popover,
          optionTextColor: foreground,
          optionTextColorActive: primary,
          optionColorPending: muted,
          optionColorActive: muted,
          groupHeaderTextColor: mutedFg,
        },
      },
    },
    Tabs: {
      colorSegment: muted,
      tabColor: muted,
      tabColorSegment: card,
      tabBorderColor: border,
      tabTextColorSegment: mutedFg,
      tabTextColorActiveSegment: foreground,
      tabTextColorHoverSegment: foreground,
      tabTextColorDisabledSegment: mutedFg,
      tabTextColorLine: mutedFg,
      tabTextColorActiveLine: primary,
      tabTextColorHoverLine: primary,
      tabTextColorBar: mutedFg,
      tabTextColorActiveBar: primary,
      tabTextColorCard: mutedFg,
      tabTextColorActiveCard: primary,
      barColor: primary,
      paneTextColor: foreground,
    },
    Tag: {
      color: muted,
      colorBordered: muted,
      textColor: foreground,
      border: `1px solid ${border}`,
      colorPrimary: primary,
      textColorPrimary: primaryFg,
      borderPrimary: `1px solid ${primary}`,
      colorSuccess: primary,
      textColorSuccess: primaryFg,
      borderSuccess: `1px solid ${primary}`,
      colorWarning: accent,
      textColorWarning: cssVar("--accent-foreground"),
      borderWarning: `1px solid ${accent}`,
      colorError: destructive,
      textColorError: cssVar("--destructive-foreground"),
      borderError: `1px solid ${destructive}`,
      closeIconColor: mutedFg,
      closeColorHover: input,
    },
    Switch: {
      railColor: muted,
      railColorActive: primary,
      buttonColor: card,
      loadingColor: primary,
      boxShadowFocus: `0 0 0 2px ${ring}`,
    },
    Checkbox: {
      color: card,
      colorChecked: primary,
      checkMarkColor: primaryFg,
      border: `1px solid ${border}`,
      borderChecked: `1px solid ${primary}`,
      borderFocus: `1px solid ${ring}`,
      boxShadowFocus: `0 0 0 2px ${ring}`,
      textColor: foreground,
    },
    Radio: {
      color: card,
      colorActive: primary,
      dotColorActive: primaryFg,
      boxShadow: `inset 0 0 0 1px ${border}`,
      boxShadowActive: `inset 0 0 0 1px ${primary}`,
      boxShadowFocus: `inset 0 0 0 1px ${primary}, 0 0 0 2px ${ring}`,
      textColor: foreground,
    },
    Progress: {
      fillColor: primary,
      railColor: muted,
      textColorCircle: foreground,
      iconColor: primary,
    },
    Spin: {
      color: primary,
      textColor: foreground,
    },
    Empty: {
      textColor: mutedFg,
      iconColor: mutedFg,
    },
    Tooltip: {
      color: popover,
      textColor: popoverFg,
    },
    Drawer: {
      color: popover,
      textColor: popoverFg,
      titleTextColor: foreground,
    },
    DataTable: {
      thColor: muted,
      tdColor: card,
      tdColorHover: muted,
      tdTextColor: foreground,
      thTextColor: mutedFg,
      borderColor: border,
    },
  };
}

const themeId = ref<ThemeId>(detectTheme());
const localeId = ref<LocaleId>(detectLocale());
const naiveOverrides = ref<GlobalThemeOverrides>({});
const naiveTheme = ref<GlobalTheme | null>(null);

function refreshNaiveTheme() {
  naiveTheme.value = isDarkSurface() ? darkTheme : null;
  naiveOverrides.value = buildNaiveOverrides();
}

export function usePreferences() {
  const themeOptions = computed(() => THEMES);

  function setTheme(id: ThemeId) {
    themeId.value = id;
    localStorage.setItem(THEME_KEY, id);
    applyThemeClass(id);
    requestAnimationFrame(() => refreshNaiveTheme());
  }

  function setLocale(id: LocaleId) {
    localeId.value = id;
    localStorage.setItem(LOCALE_KEY, id);
    document.documentElement.lang = id === "zh-CN" ? "zh-CN" : "en";
  }

  function initPreferences() {
    applyThemeClass(themeId.value);
    document.documentElement.lang = localeId.value === "zh-CN" ? "zh-CN" : "en";
    // wait a frame so CSS variables settle after class apply
    requestAnimationFrame(() => {
      requestAnimationFrame(() => refreshNaiveTheme());
    });
  }

  watch(themeId, (id) => {
    applyThemeClass(id);
    requestAnimationFrame(() => refreshNaiveTheme());
  });

  return {
    themeId,
    localeId,
    themeOptions,
    naiveTheme,
    naiveOverrides,
    setTheme,
    setLocale,
    initPreferences,
  };
}
