import { writable, get } from "svelte/store";

const STORAGE_KEY = "delveui:recency:v1";

function load(): Record<string, number> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function save(map: Record<string, number>) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(map));
  } catch {}
}

export const recency = writable<Record<string, number>>(load());

export function bumpRecency(id: string | undefined | null) {
  if (!id) return;
  recency.update((m) => {
    const next = { ...m, [id]: Date.now() };
    save(next);
    return next;
  });
}

export function getRecency(id: string): number {
  return get(recency)[id] ?? 0;
}

export function compareByRecencyThenLabel(
  aId: string,
  aLabel: string,
  bId: string,
  bLabel: string,
  map: Record<string, number>,
): number {
  const aTs = map[aId] ?? 0;
  const bTs = map[bId] ?? 0;
  if (aTs !== bTs) return bTs - aTs;
  return aLabel.localeCompare(bLabel, undefined, { sensitivity: "base" });
}
