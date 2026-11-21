import type { ThemeId } from "./types";

export const THEMES: { id: ThemeId; name: string; swatches: string[] }[] = [
  { id: "uiactf", name: "UiA CTF", swatches: ["#1a1a1a", "#262626", "#2e2e2e", "#d1002c"] },
  { id: "betauia", name: "BETA UiA", swatches: ["#0e1218", "#141922", "#1c222c", "#0085ff"] },
];

export function applyTheme(theme: ThemeId) {
  document.documentElement.dataset.theme = theme;
  try {
    localStorage.setItem("theme", theme);
  } catch {}
}
