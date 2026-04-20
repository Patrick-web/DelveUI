import { writable, get } from "svelte/store";

export type SidebarSectionId =
  | "sessions"
  | "filetree"
  | "breakpoints"
  | "callstack"
  | "threads";

export type InspectorId = "variables" | "watch" | "resources";
export type BottomId = "terminal" | "console";

export type Layout = {
  sidebarSections: Record<SidebarSectionId, { expanded: boolean }>;
  inspectorActive: InspectorId;
  bottomActive: BottomId;
  sizes: {
    sidebar: number;   // %
    inspector: number; // %
    bottom: number;    // % of center column
  };
  visible: {
    sidebar: boolean;
    inspector: boolean;
    bottom: boolean;
  };
};

const STORAGE_KEY = "delveui.layout.v7";

function defaultLayout(): Layout {
  return {
    sidebarSections: {
      sessions:    { expanded: true },
      filetree:    { expanded: true },
      breakpoints: { expanded: false },
      callstack:   { expanded: false },
      threads:     { expanded: false },
    },
    inspectorActive: "variables",
    bottomActive: "terminal",
    sizes: { sidebar: 18, inspector: 22, bottom: 30 },
    visible: { sidebar: true, inspector: true, bottom: true },
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
      sidebarSections: { ...def.sidebarSections, ...(parsed.sidebarSections ?? {}) },
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

// ---- section expand/collapse ----

export function toggleSection(id: SidebarSectionId) {
  layout.update((l) => ({
    ...l,
    sidebarSections: {
      ...l.sidebarSections,
      [id]: { expanded: !l.sidebarSections[id]?.expanded },
    },
  }));
}

export function setSectionExpanded(id: SidebarSectionId, expanded: boolean) {
  layout.update((l) => ({
    ...l,
    sidebarSections: { ...l.sidebarSections, [id]: { expanded } },
  }));
}

// ---- inspector / bottom active tab ----

export function setInspectorActive(id: InspectorId) {
  layout.update((l) => ({ ...l, inspectorActive: id }));
}

export function setBottomActive(id: BottomId) {
  layout.update((l) => ({ ...l, bottomActive: id }));
}

// ---- area visibility ----

export type AreaKey = "sidebar" | "inspector" | "bottom";

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

// ---- sizes ----

export function setAreaSize(area: AreaKey, size: number) {
  layout.update((l) => ({
    ...l,
    sizes: { ...l.sizes, [area]: size },
  }));
}

// ---- convenience ----

export function showBottomTab(id: BottomId) {
  layout.update((l) => ({
    ...l,
    bottomActive: id,
    visible: { ...l.visible, bottom: true },
  }));
}

// backward-compat shim for old callers still using toggleDock("left"|"right")
export function toggleDock(which: "left" | "right" | "bottom" | AreaKey) {
  const area: AreaKey = which === "left" ? "sidebar" : which === "right" ? "inspector" : which;
  toggleArea(area as AreaKey);
}

export function setDockVisible(which: "left" | "right" | "bottom" | AreaKey, visible: boolean) {
  const area: AreaKey = which === "left" ? "sidebar" : which === "right" ? "inspector" : which;
  setAreaVisible(area as AreaKey, visible);
}

// Shim for legacy callers. Routes panel IDs to the right area in the new
// layout regardless of the (now-meaningless) dock argument.
// - inspector panels (variables/watch/resources) → setInspectorActive + show inspector
// - bottom panels (terminal/console) → setBottomActive + show bottom
// - "source" and everything else → no-op (Source is the always-visible main area)
const INSPECTOR_IDS: ReadonlySet<InspectorId> = new Set(["variables", "watch", "resources"]);
const BOTTOM_IDS: ReadonlySet<BottomId> = new Set(["terminal", "console"]);

export function setActivePanel(_which: "left" | "right" | "bottom", panelId: string) {
  if (INSPECTOR_IDS.has(panelId as InspectorId)) {
    setInspectorActive(panelId as InspectorId);
    setAreaVisible("inspector", true);
  } else if (BOTTOM_IDS.has(panelId as BottomId)) {
    setBottomActive(panelId as BottomId);
    setAreaVisible("bottom", true);
  }
  // source / other: always-visible or unknown; ignore
}

// Legacy — removed. Kept as no-op so old imports don't explode mid-migration.
export function applyPanelSettings() { /* no-op: panels are now area-bound in registry */ }
