import { writable, get } from "svelte/store";

const STORAGE_KEY = "delveui:recency:v1";
const MAX_ENTRIES = 500;

function load(): Record<string, number> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object") return {};
    const map = parsed as Record<string, number>;
    const entries = Object.entries(map);
    if (entries.length <= MAX_ENTRIES) return map;
    entries.sort(([, a], [, b]) => b - a);
    return Object.fromEntries(entries.slice(0, MAX_ENTRIES));
  } catch {
    return {};
  }
}

function save(map: Record<string, number>) {
  try {
    const entries = Object.entries(map);
    if (entries.length > MAX_ENTRIES) {
      entries.sort(([, a], [, b]) => b - a);
      const trimmed = Object.fromEntries(entries.slice(0, MAX_ENTRIES));
      localStorage.setItem(STORAGE_KEY, JSON.stringify(trimmed));
      return;
    }
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
