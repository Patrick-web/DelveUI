import { writable, get } from "svelte/store";
import type { DockId } from "./registry";
import { PANELS } from "./registry";
import { appSettings } from "../settings-store";

type Layout = {
  assignments: Record<string, DockId>; // panelId → dockId
  active: Record<DockId, string | null>; // dockId → active panelId
  sizes: { left: number; right: number };
  visible: Record<DockId, boolean>;
};

const STORAGE_KEY = "delveui.layout.v5";

function defaultLayout(): Layout {
  const assignments: Record<string, DockId> = {};
  const activeByDock: Record<DockId, string | null> = {
    left: null,
    right: null,
  };
  for (const p of PANELS) {
    assignments[p.id] = p.defaultDock;
    if (!activeByDock[p.defaultDock]) activeByDock[p.defaultDock] = p.id;
  }
  return {
    assignments,
    active: activeByDock,
    sizes: { left: 30, right: 70 },
    visible: { left: true, right: true },
  };
}

function load(): Layout {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return defaultLayout();
    const parsed = JSON.parse(raw);
    const def = defaultLayout();
    // migrate: any panel assigned to a removed dock gets reassigned to its default
    for (const p of PANELS) {
      const assigned = parsed.assignments?.[p.id];
      if (assigned !== "left" && assigned !== "right") {
        if (parsed.assignments) parsed.assignments[p.id] = p.defaultDock;
      }
    }
    return { ...def, ...parsed, sizes: { ...def.sizes, ...parsed.sizes } };
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

// Apply panel configuration from app settings when they load
export function applyPanelSettings() {
  const s = get(appSettings);

  layout.update((l) => {
    const assignments: Record<string, DockId> = {};
    const active: Record<DockId, string | null> = { left: null, right: null };

    // Only assign panels that are in the settings lists
    for (const id of s.leftPanels ?? []) assignments[id] = "left";
    for (const id of s.rightPanels ?? []) assignments[id] = "right";

    // Panels not in either list are effectively hidden (not in assignments)

    // Set default active tabs
    const leftIds = s.leftPanels ?? [];
    const rightIds = s.rightPanels ?? [];
    active.left = (s.defaultLeftTab && leftIds.includes(s.defaultLeftTab))
      ? s.defaultLeftTab
      : leftIds[0] ?? null;
    active.right = (s.defaultRightTab && rightIds.includes(s.defaultRightTab))
      ? s.defaultRightTab
      : rightIds[0] ?? null;

    return { ...l, assignments, active };
  });
}

export function panelsInDock(l: Layout, dock: DockId): string[] {
  const s = get(appSettings);
  const ordered = dock === "left" ? s.leftPanels : s.rightPanels;

  const inDock = PANELS.filter((p) => l.assignments[p.id] === dock).map((p) => p.id);
  if (ordered?.length) {
    const set = new Set(inDock);
    return ordered.filter((id) => set.has(id));
  }
  return inDock;
}

export function setActivePanel(dock: DockId, panelId: string) {
  layout.update((l) => ({ ...l, active: { ...l.active, [dock]: panelId } }));
}

export function movePanel(panelId: string, dock: DockId) {
  layout.update((l) => {
    const assignments = { ...l.assignments, [panelId]: dock };
    // set active in target dock if empty
    const active = { ...l.active };
    if (!active[dock]) active[dock] = panelId;
    // clear active in source if it was this
    for (const d of ["left", "right", "bottom"] as DockId[]) {
      if (active[d] === panelId && d !== dock) {
        const remaining = PANELS.filter(
          (p) => assignments[p.id] === d && p.id !== panelId,
        );
        active[d] = remaining[0]?.id ?? null;
      }
    }
    return { ...l, assignments, active };
  });
}

export function toggleDock(dock: DockId) {
  layout.update((l) => ({
    ...l,
    visible: { ...l.visible, [dock]: !l.visible[dock] },
  }));
}

export function setDockVisible(dock: DockId, visible: boolean) {
  layout.update((l) => ({ ...l, visible: { ...l.visible, [dock]: visible } }));
}

export function setDockSize(dock: DockId, size: number) {
  layout.update((l) => ({ ...l, sizes: { ...l.sizes, [dock]: size } }));
}
