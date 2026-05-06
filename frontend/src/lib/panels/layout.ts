import { writable } from "svelte/store";

export type SidebarTabId = "sessions" | "debug" | "run" | "filetree" | "search";
export type CenterTabId = "source" | "terminal" | "console";
export type InspectorId = "variables" | "watch" | "callstack" | "threads" | "resources";

export type Layout = {
  sidebarActive: SidebarTabId;
  centerActive: CenterTabId;
  inspectorActive: InspectorId;
  sizes: { sidebar: number; inspector: number };
  visible: { sidebar: boolean; inspector: boolean };
  envExpanded: boolean;
  breakpointsExpanded: boolean;
};

const STORAGE_KEY = "delveui.layout.v9";

function defaultLayout(): Layout {
  return {
    sidebarActive: "sessions",
    centerActive: "source",
    inspectorActive: "variables",
    sizes: { sidebar: 18, inspector: 22 },
    visible: { sidebar: true, inspector: true },
    envExpanded: false,
    breakpointsExpanded: false,
  };
}

function load(): Layout {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return defaultLayout();
    const parsed = JSON.parse(raw);
    const def = defaultLayout();
    return {
      ...def,
      ...parsed,
      sizes: { ...def.sizes, ...(parsed.sizes ?? {}) },
      visible: { ...def.visible, ...(parsed.visible ?? {}) },
    };
  } catch {
    return defaultLayout();
  }
}

export const layout = writable<Layout>(load());

layout.subscribe((l) => {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(l));
  } catch {}
});

// ---- active tabs ----

export function setSidebarActive(id: SidebarTabId) {
  layout.update((l) => ({ ...l, sidebarActive: id }));
}

export function setCenterActive(id: CenterTabId) {
  layout.update((l) => ({ ...l, centerActive: id }));
}

export function setInspectorActive(id: InspectorId) {
  layout.update((l) => ({ ...l, inspectorActive: id }));
}

// ---- area visibility ----

export type AreaKey = "sidebar" | "inspector";

export function toggleArea(area: AreaKey) {
  layout.update((l) => ({
    ...l,
    visible: { ...l.visible, [area]: !l.visible[area] },
  }));
}

export function setAreaVisible(area: AreaKey, visible: boolean) {
  layout.update((l) => ({
    ...l,
    visible: { ...l.visible, [area]: visible },
  }));
}

export function setAreaSize(area: AreaKey, size: number) {
  layout.update((l) => ({
    ...l,
    sizes: { ...l.sizes, [area]: size },
  }));
}

// ---- env drawer ----

export function toggleEnvDrawer() {
  layout.update((l) => ({ ...l, envExpanded: !l.envExpanded }));
}

export function setEnvExpanded(expanded: boolean) {
  layout.update((l) => ({ ...l, envExpanded: expanded }));
}

// ---- breakpoints drawer ----

export function toggleBreakpointsDrawer() {
  layout.update((l) => ({ ...l, breakpointsExpanded: !l.breakpointsExpanded }));
}

export function setBreakpointsExpanded(expanded: boolean) {
  layout.update((l) => ({ ...l, breakpointsExpanded: expanded }));
}

// ---- legacy shims — route old panel ids to the right new home ----

const INSPECTOR_IDS: ReadonlySet<InspectorId> = new Set([
  "variables",
  "watch",
  "callstack",
  "threads",
  "resources",
]);
const CENTER_IDS: ReadonlySet<CenterTabId> = new Set(["source", "terminal", "console"]);
const SIDEBAR_IDS: ReadonlySet<SidebarTabId> = new Set([
  "sessions",
  "debug",
  "run",
  "filetree",
  "search",
]);

export function setActivePanel(_which: "left" | "right" | "bottom", panelId: string) {
  if (INSPECTOR_IDS.has(panelId as InspectorId)) {
    setInspectorActive(panelId as InspectorId);
    setAreaVisible("inspector", true);
  } else if (CENTER_IDS.has(panelId as CenterTabId)) {
    setCenterActive(panelId as CenterTabId);
  } else if (SIDEBAR_IDS.has(panelId as SidebarTabId)) {
    setSidebarActive(panelId as SidebarTabId);
    setAreaVisible("sidebar", true);
  } else if (panelId === "breakpoints") {
    // Breakpoints lives as a drawer in the sidebar now, not a tab.
    setBreakpointsExpanded(true);
    setAreaVisible("sidebar", true);
  }
}

export function showCenterTab(id: CenterTabId) {
  setCenterActive(id);
}

// accepted aliases: "bottom" is no longer a separate area — it means the
// terminal/console tab in the center.
export function showBottomTab(id: "terminal" | "console") {
  setCenterActive(id);
}

export function toggleDock(which: "left" | "right" | "bottom" | AreaKey) {
  if (which === "left") toggleArea("sidebar");
  else if (which === "right") toggleArea("inspector");
  else if (which === "sidebar" || which === "inspector") toggleArea(which);
  // "bottom" → no-op (no separate bottom anymore)
}

export function setDockVisible(which: "left" | "right" | "bottom" | AreaKey, visible: boolean) {
  if (which === "left") setAreaVisible("sidebar", visible);
  else if (which === "right") setAreaVisible("inspector", visible);
  else if (which === "sidebar" || which === "inspector") setAreaVisible(which, visible);
}

export function applyPanelSettings() { /* no-op */ }
