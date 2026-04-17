import { writable, get } from "svelte/store";
import * as ThemeService from "../../bindings/github.com/jp/DelveUI/internal/themes/service";
import * as SettingsService from "../../bindings/github.com/jp/DelveUI/internal/settings/service";

export type ThemeStyle = {
  bg: string;
  bgElevated: string;
  bgSubtle: string;
  surface: string;
  text: string;
  textMuted: string;
  textFaint: string;
  border: string;
  borderSubtle: string;
  accent: string;
  accentSubtle: string;
  danger: string;
  warning: string;
  success: string;
  info: string;
  synKeyword: string;
  synString: string;
  synNumber: string;
  synFn: string;
  synComment: string;
  terminal: Record<string, string>;
};

export type ThemeDefinition = {
  name: string;
  author: string;
  appearance: string;
  style: ThemeStyle;
};

export type ThemeMeta = {
  name: string;
  author: string;
  appearance: string;
  bundled: boolean;
};

export const currentThemeName = writable<string>("One Dark");
export const currentTheme = writable<ThemeDefinition | null>(null);
export const themeList = writable<ThemeMeta[]>([]);

function camelToKebab(s: string): string {
  return s.replace(/[A-Z]/g, (m) => "-" + m.toLowerCase());
}

export function applyTheme(theme: ThemeDefinition) {
  const root = document.documentElement;
  const style = theme.style as any;
  for (const [key, value] of Object.entries(style)) {
    if (key === "terminal") continue;
    const prop = "--" + camelToKebab(key);
    root.style.setProperty(prop, value as string);
  }
  if (style.terminal) {
    for (const [key, value] of Object.entries(style.terminal as Record<string, string>)) {
      root.style.setProperty("--term-" + camelToKebab(key), value);
    }
  }
  // Update meta theme-color for native feeling
  const meta = document.querySelector('meta[name="theme-color"]');
  if (meta) meta.setAttribute("content", theme.style.bg);
  currentTheme.set(theme);
  currentThemeName.set(theme.name);
}

export async function loadTheme(name: string) {
  try {
    const theme = (await ThemeService.Get(name)) as any as ThemeDefinition;
    applyTheme(theme);
  } catch (e) {
    console.error("Failed to load theme:", name, e);
  }
}

// Preview applies CSS vars without updating stores (for hover preview).
export async function previewThemeByName(name: string) {
  try {
    const theme = (await ThemeService.Get(name)) as any as ThemeDefinition;
    applyThemeCSS(theme);
  } catch {}
}

// Apply only CSS vars, no store update.
function applyThemeCSS(theme: ThemeDefinition) {
  const root = document.documentElement;
  const style = theme.style as any;
  for (const [key, value] of Object.entries(style)) {
    if (key === "terminal") continue;
    root.style.setProperty("--" + camelToKebab(key), value as string);
  }
  if (style.terminal) {
    for (const [key, value] of Object.entries(style.terminal as Record<string, string>)) {
      root.style.setProperty("--term-" + camelToKebab(key), value);
    }
  }
}

// Revert preview by re-applying the saved theme from stores.
export async function revertThemePreview() {
  const saved = get(currentThemeName);
  if (saved) {
    try {
      const theme = (await ThemeService.Get(saved)) as any as ThemeDefinition;
      applyThemeCSS(theme);
    } catch {}
  }
}

export async function refreshThemeList() {
  try {
    const list = (await ThemeService.List()) as any as ThemeMeta[];
    themeList.set(list ?? []);
  } catch (e) {
    console.error("Failed to list themes:", e);
  }
}

export async function initTheme() {
  try {
    const settings = (await SettingsService.Get()) as any;
    const name = settings?.theme || "One Dark";
    await refreshThemeList();
    await loadTheme(name);
  } catch (e) {
    console.error("Theme init failed:", e);
  }
}

export async function setTheme(name: string) {
  await loadTheme(name);
  try {
    const settings = (await SettingsService.Get()) as any;
    settings.theme = name;
    await SettingsService.Update(settings);
  } catch (e) {
    console.error("Failed to save theme preference:", e);
  }
}

export async function installThemeFromFile() {
  try {
    const meta = (await ThemeService.ImportFile("")) as any;
    if (meta?.name) {
      await refreshThemeList();
      return meta as ThemeMeta;
    }
  } catch (e) {
    console.error("Theme install failed:", e);
  }
  return null;
}

export async function removeTheme(name: string) {
  try {
    await ThemeService.Remove(name);
    await refreshThemeList();
    if (get(currentThemeName) === name) {
      await setTheme("One Dark");
    }
  } catch (e) {
    console.error("Theme remove failed:", e);
  }
}
