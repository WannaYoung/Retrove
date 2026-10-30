import { createI18n } from "vue-i18n";
import zhCN from "./zh-CN";
import enUS from "./en-US";

function initialLocale(): "zh-CN" | "en-US" {
  const saved = localStorage.getItem("retrove.locale");
  if (saved === "zh-CN" || saved === "en-US") return saved;
  return navigator.language.toLowerCase().startsWith("zh") ? "zh-CN" : "en-US";
}

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale(),
  fallbackLocale: "en-US",
  messages: {
    "zh-CN": zhCN,
    "en-US": enUS,
  },
});
