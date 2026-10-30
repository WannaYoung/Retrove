export type ThemeId =
  | "homebox"
  | "aqua"
  | "black"
  | "bumblebee"
  | "cmyk"
  | "corporate"
  | "cupcake"
  | "cyberpunk"
  | "dracula"
  | "emerald"
  | "fantasy"
  | "forest"
  | "garden"
  | "halloween"
  | "lemonade"
  | "light"
  | "lofi"
  | "luxury"
  | "night"
  | "pastel"
  | "retro"
  | "synthwave"
  | "valentine"
  | "winter"
  | "wireframe"
  | "acid"
  | "autumn"
  | "business"
  | "coffee";

export type ThemeMeta = {
  id: ThemeId;
  /** CSS class on <html>; homebox uses .homebox */
  cssClass: string;
  nameKey: string;
};

export const THEMES: ThemeMeta[] = [
  { id: "homebox", cssClass: "homebox", nameKey: "theme.homebox" },
  { id: "garden", cssClass: "theme-garden", nameKey: "theme.garden" },
  { id: "emerald", cssClass: "theme-emerald", nameKey: "theme.emerald" },
  { id: "forest", cssClass: "theme-forest", nameKey: "theme.forest" },
  { id: "aqua", cssClass: "theme-aqua", nameKey: "theme.aqua" },
  { id: "lemonade", cssClass: "theme-lemonade", nameKey: "theme.lemonade" },
  { id: "bumblebee", cssClass: "theme-bumblebee", nameKey: "theme.bumblebee" },
  { id: "autumn", cssClass: "theme-autumn", nameKey: "theme.autumn" },
  { id: "coffee", cssClass: "theme-coffee", nameKey: "theme.coffee" },
  { id: "halloween", cssClass: "theme-halloween", nameKey: "theme.halloween" },
  { id: "valentine", cssClass: "theme-valentine", nameKey: "theme.valentine" },
  { id: "cupcake", cssClass: "theme-cupcake", nameKey: "theme.cupcake" },
  { id: "pastel", cssClass: "theme-pastel", nameKey: "theme.pastel" },
  { id: "light", cssClass: "theme-light", nameKey: "theme.light" },
  { id: "corporate", cssClass: "theme-corporate", nameKey: "theme.corporate" },
  { id: "business", cssClass: "theme-business", nameKey: "theme.business" },
  { id: "winter", cssClass: "theme-winter", nameKey: "theme.winter" },
  { id: "luxury", cssClass: "theme-luxury", nameKey: "theme.luxury" },
  { id: "night", cssClass: "theme-night", nameKey: "theme.night" },
  { id: "black", cssClass: "theme-black", nameKey: "theme.black" },
  { id: "dracula", cssClass: "theme-dracula", nameKey: "theme.dracula" },
  { id: "synthwave", cssClass: "theme-synthwave", nameKey: "theme.synthwave" },
  { id: "cyberpunk", cssClass: "theme-cyberpunk", nameKey: "theme.cyberpunk" },
  { id: "acid", cssClass: "theme-acid", nameKey: "theme.acid" },
  { id: "fantasy", cssClass: "theme-fantasy", nameKey: "theme.fantasy" },
  { id: "retro", cssClass: "theme-retro", nameKey: "theme.retro" },
  { id: "cmyk", cssClass: "theme-cmyk", nameKey: "theme.cmyk" },
  { id: "lofi", cssClass: "theme-lofi", nameKey: "theme.lofi" },
  { id: "wireframe", cssClass: "theme-wireframe", nameKey: "theme.wireframe" },
];

export const DEFAULT_THEME: ThemeId = "retro";

export function isThemeId(value: string): value is ThemeId {
  return THEMES.some((t) => t.id === value);
}
