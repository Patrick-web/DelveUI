<script lang="ts">
  import { activeSession, activeSessionId } from "./store";
  import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";
  import Icon from "./Icon.svelte";

  type Entry = { key: string; value: string };

  let loading = false;
  let entries: Entry[] = [];
  let error = "";
  let filter = "";

  function fuzzy(q: string, s: string): boolean {
    if (!q) return true;
    const hay = s.toLowerCase();
    const needle = q.toLowerCase();
    let i = 0;
    for (const c of hay) { if (c === needle[i]) i++; if (i === needle.length) return true; }
    return false;
  }

  $: visibleEntries = filter ? entries.filter((e) => fuzzy(filter, e.key) || fuzzy(filter, e.value)) : entries;
  // Files contributing to the current view, in precedence order. Set when
  // the cfg has envFiles[] (discovery walk-up) or a legacy single envFile.
  let sourceFiles: string[] = [];

  // Reload whenever the active session changes
  $: cfg = ($activeSession as any)?.cfg;
  $: {
    const inline = (cfg?.env ?? {}) as Record<string, string>;
    const files: string[] = (cfg?.envFiles && cfg.envFiles.length > 0)
      ? cfg.envFiles
      : (cfg?.envFile ? [cfg.envFile] : []);
    const cwd = cfg?.cwd ?? "";
    loadEnv(inline, files, cwd);
  }

  // Loader: starts with inline env (already merged on the backend for Run
  // targets), then layers each source file on top so the inspector matches
  // what the process actually sees. Last file wins for collisions.
  async function loadEnv(inline: Record<string, string>, files: string[], cwd: string) {
    entries = [];
    error = "";
    sourceFiles = [];
    loading = true;
    try {
      const merged: Record<string, string> = { ...inline };
      const resolved: string[] = [];
      for (const p of files) {
        const text = await tryRead(p, cwd);
        if (text == null) continue;
        resolved.push(text.path);
        for (const e of parseDotenv(text.text)) merged[e.key] = e.value;
      }
      sourceFiles = resolved;
      entries = Object.entries(merged)
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([k, v]) => ({ key: k, value: v }));
    } catch (e: any) {
      error = String(e?.message ?? e);
    } finally {
      loading = false;
    }
  }

  async function tryRead(envPath: string, cwd: string): Promise<{ path: string; text: string } | null> {
    try {
      const text = (await FileService.ReadFile(envPath)) as string;
      return { path: envPath, text };
    } catch {
      if (cwd && !envPath.startsWith("/")) {
        const abs = cwd.replace(/\/+$/, "") + "/" + envPath;
        try {
          const text = (await FileService.ReadFile(abs)) as string;
          return { path: abs, text };
        } catch { /* fall through */ }
      }
    }
    return null;
  }

  function parseDotenv(text: string): Entry[] {
    const out: Entry[] = [];
    for (const rawLine of text.split(/\r?\n/)) {
      const line = rawLine.trim();
      if (!line || line.startsWith("#")) continue;
      const body = line.replace(/^export\s+/, "");
      const eq = body.indexOf("=");
      if (eq < 0) continue;
      const key = body.slice(0, eq).trim();
      let value = body.slice(eq + 1).trim();
      if (
        (value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'"))
      ) {
        value = value.slice(1, -1);
      }
      if (key) out.push({ key, value });
    }
    return out;
  }

  function short(p: string): string {
    if (!p) return "";
    return p.replace(/^\/Users\/[^/]+/, "~").split("/").slice(-3).join("/");
  }

  async function copyValue(v: string) {
    try { await navigator.clipboard.writeText(v); } catch {}
  }

  $: hasInline = !!cfg?.env && Object.keys(cfg.env).length > 0;
  $: hasFiles = sourceFiles.length > 0 || (cfg?.envFiles?.length ?? 0) > 0 || !!cfg?.envFile;
  $: hasContent = entries.length > 0 || hasInline || hasFiles;
</script>

<div class="env">
  {#if !$activeSessionId}
    <div class="empty">No active session.</div>
  {:else if loading}
    <div class="empty">Loading…</div>
  {:else if error}
    <div class="empty err">{error}</div>
  {:else if !hasContent}
    <div class="empty">This session has no environment variables set.</div>
  {:else}
    <div class="sub-head">
      <Icon icon="solar:settings-linear" size={11} color="var(--text-faint)" />
      <span class="path">
        {#if sourceFiles.length === 0 && hasInline}
          Inline env from launch config
        {:else if sourceFiles.length === 1}
          {short(sourceFiles[0])}
        {:else if sourceFiles.length > 1}
          {sourceFiles.length} env files merged
        {/if}
      </span>
      <span class="count">{entries.length} {entries.length === 1 ? "entry" : "entries"}</span>
    </div>
    {#if sourceFiles.length > 1}
      <div class="sources">
        {#each sourceFiles as f, i}
          <div class="source" title={f}>
            <span class="src-idx">{i + 1}.</span>
            <Icon icon="solar:document-text-linear" size={10} color="var(--text-faint)" />
            <span class="src-path">{short(f)}</span>
          </div>
        {/each}
        <div class="src-note">Later files override earlier ones.</div>
      </div>
    {/if}
    <div class="env-filter">
      <Icon icon="solar:magnifer-linear" size={12} color="var(--text-faint)" />
      <input class="env-filter-input" placeholder="Filter…" bind:value={filter} spellcheck="false" />
      {#if filter}
        <button class="env-clear" title="Clear" on:click={() => (filter = "")}>
          <Icon icon="solar:close-circle-linear" size={12} />
        </button>
      {/if}
    </div>
    {#if visibleEntries.length === 0}
      <div class="empty">{entries.length > 0 ? "No matching entries." : "No KEY=value entries."}</div>
    {:else}
      <div class="rows">
        {#each visibleEntries as e}
          <div class="row" title="Click to copy value" on:click={() => copyValue(e.value)} on:keydown={(ev) => { if (ev.key === 'Enter') copyValue(e.value); }} role="button" tabindex="0">
            <span class="k">{e.key}</span>
            <span class="v">{e.value}</span>
          </div>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  .env {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }
  .empty {
    padding: var(--space-3);
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  .empty code {
    font-family: var(--font-mono);
    background: var(--bg-subtle);
    padding: 1px 5px;
    border-radius: 3px;
  }
  .empty.err { color: var(--danger); font-family: var(--font-mono); }

  .sub-head {
    display: flex;
    align-items: center;
    gap: 6px;
    height: 22px;
    padding: 0 10px;
    background: var(--bg-subtle);
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }
  .sub-head .path {
    flex: 1;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sub-head .count {
    font-size: 10px;
    color: var(--text-faint);
    font-family: var(--font-mono);
  }

  .sources {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 4px 10px 6px 10px;
    background: var(--bg-subtle);
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }
  .source {
    display: flex;
    align-items: center;
    gap: 4px;
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-muted);
  }
  .src-idx { color: var(--text-faint); width: 14px; }
  .src-path { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .src-note { color: var(--text-faint); font-size: 9px; padding-top: 2px; }

  .env-filter {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 6px 6px 4px 6px;
    padding: 0 8px;
    height: 26px;
    background: var(--bg);
    border: 1px solid var(--border-subtle);
    border-radius: 5px;
    flex-shrink: 0;
  }
  .env-filter:focus-within { border-color: var(--accent); }
  .env-filter-input {
    flex: 1;
    background: transparent;
    border: 0;
    color: var(--text);
    font-size: var(--text-xs);
    font-family: var(--font-ui);
    outline: none;
    padding: 0;
  }
  .env-clear {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 0;
    color: var(--text-faint);
    cursor: pointer;
    padding: 0;
    width: 16px;
    height: 16px;
  }
  .env-clear:hover { color: var(--text); }

  .rows {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 4px 0;
  }
  .row {
    display: flex;
    gap: 8px;
    padding: 3px 10px;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    cursor: pointer;
    border-radius: 3px;
    margin: 0 4px;
  }
  .row:hover { background: var(--bg-subtle); }
  .k {
    flex-shrink: 0;
    max-width: 45%;
    color: var(--syn-keyword);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .v {
    flex: 1;
    color: var(--syn-string);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
