import { writable } from "svelte/store";
import * as SettingsService from "../../bindings/github.com/jp/DelveUI/internal/settings/service";
import * as DebugFilesStore from "../../bindings/github.com/jp/DelveUI/internal/debugfiles/store";

export type AppSettings = {
  theme: string;
  terminalTheme: string;
  vimMode: boolean;
  uiFontSize: number;
  bufferFontSize: number;
  termFontSize: number;
  lineHeight: string;
  leftPanels: string[];
  rightPanels: string[];
  defaultLeftTab: string;
  defaultRightTab: string;
};

export type DebugFileEntry = {
  id: string;
  path: string;
  label: string;
  isDefault: boolean;
  addedAt: string;
  configs: any[];
};

export const appSettings = writable<AppSettings>({
  theme: "One Dark",
  terminalTheme: "follow",
  vimMode: false,
  uiFontSize: 13,
  bufferFontSize: 13,
  termFontSize: 12,
  lineHeight: "standard",
  leftPanels: ["breakpoints", "callstack", "threads", "variables", "resources"],
  rightPanels: ["terminal", "console"],
  defaultLeftTab: "breakpoints",
  defaultRightTab: "terminal",
});

export const debugFiles = writable<DebugFileEntry[]>([]);

export async function loadSettings() {
  try {
    const s = (await SettingsService.Get()) as any as AppSettings;
    appSettings.set(s);
  } catch (e) {
    console.error("Failed to load settings:", e);
  }
}

export async function saveSettings(s: AppSettings) {
  appSettings.set(s);
  try {
    await SettingsService.Update(s as any);
  } catch (e) {
    console.error("Failed to save settings:", e);
  }
}

export async function loadDebugFiles() {
  try {
    const list = (await DebugFilesStore.List()) as any as DebugFileEntry[];
    debugFiles.set(list ?? []);
  } catch (e) {
    console.error("Failed to load debug files:", e);
  }
}

export async function addDebugFile(path: string) {
  try {
    await DebugFilesStore.Add(path);
    await loadDebugFiles();
  } catch (e) {
    console.error("Failed to add debug file:", e);
    throw e;
  }
}

export async function removeDebugFile(id: string) {
  try {
    await DebugFilesStore.Remove(id);
    await loadDebugFiles();
  } catch (e) {
    console.error("Failed to remove debug file:", e);
  }
}

export async function setDefaultDebugFile(id: string) {
  try {
    await DebugFilesStore.SetDefault(id);
    await loadDebugFiles();
  } catch (e) {
    console.error("Failed to set default:", e);
  }
}

export async function reloadDebugFile(id: string) {
  try {
    await DebugFilesStore.Reload(id);
    await loadDebugFiles();
  } catch (e) {
    console.error("Failed to reload:", e);
  }
}
