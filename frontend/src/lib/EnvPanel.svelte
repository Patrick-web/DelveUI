<script lang="ts">
  import { activeSession, activeSessionId } from "./store";
  import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";
  import Icon from "./Icon.svelte";

  type Entry = { key: string; value: string };

  let loading = false;
  let path = "";
  let entries: Entry[] = [];
  let rawText = "";
  let error = "";

  // Reload whenever the active session changes
  $: cfg = ($activeSession as any)?.cfg;
  $: {
    const p = cfg?.envFile ?? "";
    const cwd = cfg?.cwd ?? "";
    loadEnv(p, cwd);
  }

  async function loadEnv(envPath: string, cwd: string) {
    entries = [];
    error = "";
    rawText = "";
    path = envPath ?? "";
    if (!envPath) return;
    loading = true;
    try {
      const tried: string[] = [envPath];
      let text: string | null = null;
      try {
        text = (await FileService.ReadFile(envPath)) as string;
      } catch {
        // Retry relative to cwd if the first attempt failed
        if (cwd && !envPath.startsWith("/")) {
          const abs = cwd.replace(/\/+$/, "") + "/" + envPath;
          tried.push(abs);
          try {
            text = (await FileService.ReadFile(abs)) as string;
            path = abs;
          } catch {
            text = null;
          }
        }
      }
      if (text === null) {
        error = "Could not read " + tried.join(" or ");
        return;
      }
      rawText = text;
      entries = parseDotenv(text);
    } catch (e: any) {
      error = String(e?.message ?? e);
    } finally {
      loading = false;
    }
  }

  function parseDotenv(text: string): Entry[] {
    const out: Entry[] = [];
    for (const rawLine of text.split(/\r?\n/)) {
      const line = rawLine.trim();
      if (!line || line.startsWith("#")) continue;
      // Strip optional "export " prefix
      const body = line.replace(/^export\s+/, "");
      const eq = body.indexOf("=");
      if (eq < 0) continue;
      const key = body.slice(0, eq).trim();
      let value = body.slice(eq + 1).trim();
      // Strip matching single or double quotes
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
</script>

<div class="env">
  {#if !$activeSessionId}
    <div class="empty">No active session.</div>
  {:else if !cfg?.envFile}
    <div class="empty">This config has no <code>envFile</code> set.</div>
  {:else if loading}
    <div class="empty">Loading…</div>
  {:else if error}
    <div class="empty err">{error}</div>
  {:else}
    <div class="sub-head">
      <Icon icon="solar:file-text-linear" size={11} color="var(--text-faint)" />
      <span class="path" title={path}>{short(path)}</span>
      <span class="count">{entries.length} {entries.length === 1 ? "entry" : "entries"}</span>
    </div>
    {#if entries.length === 0}
      <div class="empty">File loaded but no KEY=value entries found.</div>
    {:else}
      <div class="rows">
        {#each entries as e}
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
