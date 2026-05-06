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
  dlvPath?: string;
  // restoreLastProject controls whether the most-recently-active project is
  // reopened on launch. Defaults to true. Backend uses *bool so the JSON file
  // can omit it; frontend treats undefined as true.
  restoreLastProject?: boolean;
  leftPanels: string[];
  rightPanels: string[];
  defaultLeftTab: string;
  defaultRightTab: string;
};

export type DebugFileEntry = {
  id: string;
  path: string;
  label: string;
  // launchFile is set only when the entry was originally registered by
  // pointing at a launch.json that lives outside .zed/.vscode/.delveui — it
  // overrides folder-walk discovery. Empty/undefined means the standard
  // folder lookup is used.
  launchFile?: string;
  addedAt: string;
  lastUsed?: string;
  configs: any[];
  // stale = true when the path no longer exists on disk; backend marks this
  // at load time. Frontend dims the row and offers a remove action.
  stale?: boolean;
};

export const appSettings = writable<AppSettings>({
  theme: "One Dark",
  terminalTheme: "follow",
  vimMode: false,
  uiFontSize: 13,
  bufferFontSize: 13,
  termFontSize: 12,
  lineHeight: "standard",
  restoreLastProject: true,
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
    applyFontSettingsGlobal(s);
  } catch (e) {
    console.error("Failed to load settings:", e);
  }
}

function applyFontSettingsGlobal(s: AppSettings) {
  const root = document.documentElement;
  if (s.uiFontSize) root.style.setProperty("--text-md", s.uiFontSize + "px");
  if (s.bufferFontSize) root.style.setProperty("--text-sm", s.bufferFontSize + "px");
  if (s.termFontSize) root.style.setProperty("--text-term", s.termFontSize + "px");
  if (s.lineHeight) {
    const lh = s.lineHeight === "compact" ? "1.2" : s.lineHeight === "comfortable" ? "1.618" : "1.3";
    root.style.setProperty("--lh-standard", lh);
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

// removeStaleDebugFiles drops every entry whose folder is missing on disk.
// Returns the number removed so the UI can confirm with the user.
export async function removeStaleDebugFiles(): Promise<number> {
  try {
    const n = (await DebugFilesStore.RemoveStale()) as any as number;
    await loadDebugFiles();
    return n ?? 0;
  } catch (e) {
    console.error("Failed to remove stale entries:", e);
    return 0;
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
