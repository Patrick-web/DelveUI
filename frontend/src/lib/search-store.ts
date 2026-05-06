import { writable, get } from "svelte/store";
import * as SearchService from "../../bindings/github.com/jp/DelveUI/internal/search/service";

export type SearchRange = [number, number];

export type SearchMatch = {
  path: string;
  rel: string;
  line: number;
  col: number;
  text: string;
  ranges: SearchRange[];
};

export type SearchOptions = {
  query: string;
  regex: boolean;
  caseSensitive: boolean;
  wholeWord: boolean;
  includes: string;
  excludes: string;
};

export type SearchStatus = "idle" | "searching" | "done" | "error";

export type SearchSummary = {
  files: number;
  matches: number;
  durationMs: number;
  truncated: boolean;
  cancelled: boolean;
};

export type SearchState = {
  options: SearchOptions;
  status: SearchStatus;
  currentId: string;
  results: SearchMatch[];
  summary: SearchSummary | null;
  errorMessage: string;
  // Files the user has collapsed in the results tree, keyed by path.
  collapsedFiles: Record<string, true>;
};

const STORAGE_KEY = "delveui.search.v1";

function defaultState(): SearchState {
  return {
    options: { query: "", regex: false, caseSensitive: false, wholeWord: false, includes: "", excludes: "" },
    status: "idle",
    currentId: "",
    results: [],
    summary: null,
    errorMessage: "",
    collapsedFiles: {},
  };
}

function load(): SearchState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return defaultState();
    const parsed = JSON.parse(raw);
    const def = defaultState();
    return {
      ...def,
      ...parsed,
      options: { ...def.options, ...(parsed.options ?? {}) },
      // Don't restore in-flight state across reloads.
      status: "idle",
      currentId: "",
      // Results are persisted so switching tabs keeps them, but a hard reload
      // clears them — they'd be stale anyway if the user edited files.
      results: [],
      summary: null,
      errorMessage: "",
    };
  } catch {
    return defaultState();
  }
}

export const searchState = writable<SearchState>(load());

// Persist only the inputs (not active results) so tabs survive but reloads start fresh.
searchState.subscribe((s) => {
  try {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ options: s.options, collapsedFiles: s.collapsedFiles }),
    );
  } catch {}
});

export function setSearchOptions(patch: Partial<SearchOptions>) {
  searchState.update((s) => ({ ...s, options: { ...s.options, ...patch } }));
}

export function clearSearchResults() {
  searchState.update((s) => ({
    ...s,
    status: "idle",
    currentId: "",
    results: [],
    summary: null,
    errorMessage: "",
  }));
}

export function toggleFileCollapsed(path: string) {
  searchState.update((s) => {
    const next = { ...s.collapsedFiles };
    if (next[path]) delete next[path];
    else next[path] = true;
    return { ...s, collapsedFiles: next };
  });
}

function parseGlobs(s: string): string[] {
  return s
    .split(/[,\s]+/)
    .map((x) => x.trim())
    .filter((x) => x.length > 0);
}

// Token bumped on every runSearch call so a slow-resolving older call can't
// overwrite the currentId after a newer one has taken over.
let runToken = 0;

// Run the current options against the backend. Cancels any prior search.
export async function runSearch(): Promise<void> {
  const myToken = ++runToken;
  const s = get(searchState);
  const q = s.options.query.trim();
  if (!q) {
    clearSearchResults();
    return;
  }

  // Cancel any in-flight request before starting a new one. The backend also
  // cancels the prior context, but doing it here removes a race where the
  // old `done` event lands after we set `status = "searching"`.
  if (s.currentId) {
    try { await SearchService.Cancel(s.currentId); } catch {}
  }

  searchState.update((st) => ({
    ...st,
    status: "searching",
    results: [],
    summary: null,
    errorMessage: "",
  }));

  try {
    const id = await SearchService.Search({
      query: q,
      root: "",
      regex: s.options.regex,
      caseSensitive: s.options.caseSensitive,
      wholeWord: s.options.wholeWord,
      includes: parseGlobs(s.options.includes),
      excludes: parseGlobs(s.options.excludes),
      maxResults: 2000,
    } as any);
    if (myToken !== runToken) {
      // A newer runSearch superseded us; cancel the id we just got and drop it.
      try { await SearchService.Cancel(id); } catch {}
      return;
    }
    searchState.update((st) => ({ ...st, currentId: id }));
  } catch (e: any) {
    if (myToken !== runToken) return;
    searchState.update((st) => ({
      ...st,
      status: "error",
      errorMessage: String(e?.message ?? e),
    }));
  }
}

export async function cancelSearch(): Promise<void> {
  const s = get(searchState);
  if (!s.currentId) return;
  // Eagerly mark as done/cancelled so the UI flips immediately. The backend
  // 'done' event will arrive later; the id-match guard in the handler keeps
  // it from re-overwriting state.
  searchState.update((st) => ({
    ...st,
    status: "done",
    summary: { files: 0, matches: st.results.length, durationMs: 0, truncated: false, cancelled: true },
    currentId: "",
  }));
  try { await SearchService.Cancel(s.currentId); } catch {}
}

// Wire the streaming events. Called once at app startup; idempotent.
let wired = false;
export async function initSearchEvents(): Promise<void> {
  if (wired) return;
  wired = true;
  const { Events } = await import("@wailsio/runtime");

  Events.On("search:match", (ev: any) => {
    const d = ev?.data ?? ev ?? {};
    const id: string = d.id;
    const batch: SearchMatch[] = d.matches ?? [];
    if (!id || !batch.length) return;
    searchState.update((s) => {
      if (s.currentId !== id) return s; // stale
      return { ...s, results: [...s.results, ...batch] };
    });
  });

  Events.On("search:done", (ev: any) => {
    const d = ev?.data ?? ev ?? {};
    const id: string = d.id;
    if (!id) return;
    searchState.update((s) => {
      if (s.currentId !== id) return s;
      return {
        ...s,
        status: "done",
        currentId: "",
        summary: {
          files: d.files ?? 0,
          matches: d.matches ?? 0,
          durationMs: d.durationMs ?? 0,
          truncated: !!d.truncated,
          cancelled: !!d.cancelled,
        },
      };
    });
  });

  Events.On("search:error", (ev: any) => {
    const d = ev?.data ?? ev ?? {};
    const id: string = d.id;
    if (!id) return;
    searchState.update((s) => {
      if (s.currentId !== id) return s;
      return {
        ...s,
        status: "error",
        currentId: "",
        errorMessage: d.message ?? "Search failed",
      };
    });
  });
}

// Group flat match list by file path for the tree view.
export type SearchFileGroup = {
  path: string;
  rel: string;
  matches: SearchMatch[];
};

export function groupByFile(matches: SearchMatch[]): SearchFileGroup[] {
  const map = new Map<string, SearchFileGroup>();
  for (const m of matches) {
    let g = map.get(m.path);
    if (!g) {
      g = { path: m.path, rel: m.rel, matches: [] };
      map.set(m.path, g);
    }
    g.matches.push(m);
  }
  return Array.from(map.values());
}
