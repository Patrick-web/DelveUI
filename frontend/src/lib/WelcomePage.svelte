<script lang="ts">
  import { onMount } from "svelte";
  import Icon from "./Icon.svelte";
  import {
    debugFiles,
    loadDebugFiles,
    removeStaleDebugFiles,
    type DebugFileEntry,
  } from "./settings-store";
  import {
    pickWorkspaceFolder,
    pickDebugFile,
    refreshWorkspace,
    refreshTargets,
    openDebugFile,
  } from "./store";

  export let visible = false;
  export let onDone: () => void = () => {};

  let recents: DebugFileEntry[] = [];
  let loadingRecents = true;

  async function refreshRecents() {
    await loadDebugFiles();
    recents = ($debugFiles ?? [])
      .slice()
      .sort((a, b) => {
        if (!!a.stale !== !!b.stale) return a.stale ? 1 : -1;
        const at = new Date(a.lastUsed ?? a.addedAt).getTime() || 0;
        const bt = new Date(b.lastUsed ?? b.addedAt).getTime() || 0;
        return bt - at;
      });
    loadingRecents = false;
  }

  onMount(() => { if (visible) refreshRecents(); });
  $: if (visible) refreshRecents();

  async function openFolder() {
    await pickWorkspaceFolder();
    await refreshWorkspace();
    refreshTargets().catch(() => {});
    onDone();
  }

  async function openDebugJson() {
    await pickDebugFile();
    await refreshWorkspace();
    refreshTargets().catch(() => {});
    onDone();
  }

  async function openRecent(e: DebugFileEntry) {
    if (e.stale) return;
    await openDebugFile(e.path);
    refreshTargets().catch(() => {});
    onDone();
  }

  async function cleanStale() {
    const n = await removeStaleDebugFiles();
    await refreshRecents();
    if (n > 0) {
      const { showInfo } = await import("./toast");
      showInfo(`Removed ${n} missing project${n === 1 ? "" : "s"}`, "");
    }
  }

  function shortPath(p: string) { return p.replace(/^\/Users\/[^/]+/, "~"); }

  function fmtRecency(d?: string): string {
    if (!d) return "";
    const ts = new Date(d).getTime();
    if (!ts) return "";
    const sec = Math.max(0, Math.floor((Date.now() - ts) / 1000));
    if (sec < 60) return "just now";
    if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
    if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
    if (sec < 86400 * 30) return `${Math.floor(sec / 86400)}d ago`;
    return new Date(ts).toLocaleDateString();
  }

  $: hasStale = recents.some((e) => e.stale);
</script>

{#if visible}
  <div class="welcome">
    <div class="container">
      <div class="header">
        <Icon icon="solar:code-2-bold-duotone" size={36} color="var(--accent)" />
        <div>
          <h1>Welcome to DelveUI</h1>
          <p class="subtitle">Open a folder to start debugging Go.</p>
        </div>
      </div>

      <!-- Primary CTAs: pick a folder or a debug.json directly. -->
      <div class="primary-actions">
        <button class="cta primary" on:click={openFolder}>
          <Icon icon="solar:folder-with-files-bold" size={16} />
          <span class="cta-label">
            <span class="cta-title">Open Folder…</span>
            <span class="cta-sub">Pick the project you're working on</span>
          </span>
        </button>
        <button class="cta secondary" on:click={openDebugJson}>
          <Icon icon="solar:document-add-bold" size={16} />
          <span class="cta-label">
            <span class="cta-title">Open debug.json…</span>
            <span class="cta-sub">Point at an existing debug.json or launch.json file</span>
          </span>
        </button>
      </div>

      {#if loadingRecents}
        <div class="recents-empty">
          <div class="spinner"></div>
          <span>Loading recent projects…</span>
        </div>
      {:else if recents.length > 0}
        <div class="section">
          <div class="section-head">
            <span class="section-title">Recent</span>
            {#if hasStale}
              <button class="link" on:click={cleanStale}>Remove missing</button>
            {/if}
          </div>
          <div class="recent-list">
            {#each recents.slice(0, 8) as e (e.id)}
              <button
                class="recent"
                class:stale={e.stale}
                title={e.stale ? `Folder missing: ${e.path}` : e.path}
                on:click={() => openRecent(e)}
                disabled={e.stale}
              >
                <Icon icon="solar:folder-open-bold" size={14} color={e.stale ? "var(--text-faint)" : "var(--accent)"} />
                <div class="recent-info">
                  <div class="recent-title">
                    <span class="recent-name">{e.label}</span>
                    {#if e.stale}<span class="missing">missing</span>{/if}
                    {#if e.configs?.length}
                      <span class="recent-cfg">{e.configs.length} cfg{e.configs.length !== 1 ? "s" : ""}</span>
                    {/if}
                  </div>
                  <div class="recent-path">{shortPath(e.path)}</div>
                </div>
                <span class="recent-time">{fmtRecency(e.lastUsed ?? e.addedAt)}</span>
              </button>
            {/each}
          </div>
        </div>
      {/if}

      <div class="footer">
        <button class="link" on:click={onDone}>Skip</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .welcome { position:fixed; inset:0; z-index:700; background:var(--bg); display:flex; align-items:flex-start; justify-content:center; overflow:auto; }
  .container { width:100%; max-width:620px; padding:48px 24px; display:flex; flex-direction:column; gap:24px; }

  .header { display:flex; align-items:center; gap:14px; }
  h1 { font-size:22px; font-weight:700; color:var(--text); margin:0; }
  .subtitle { color:var(--text-muted); font-size:14px; margin:0; }

  .primary-actions { display:flex; flex-direction:column; gap:8px; }
  .cta {
    display:flex; align-items:center; gap:14px;
    width:100%; padding:14px 16px;
    background:var(--accent); color:#fff;
    border:1px solid var(--accent);
    border-radius:10px;
    cursor:pointer;
    text-align:left;
    transition:background 80ms ease, transform 80ms ease;
  }
  .cta:hover { background:#5ea6ff; }
  .cta.secondary {
    background:transparent;
    border-color:var(--border);
    color:var(--text);
  }
  .cta.secondary:hover { background:rgba(255,255,255,0.04); border-color:var(--accent); }
  .cta.secondary .cta-sub { color:var(--text-faint); }
  .cta-label { display:flex; flex-direction:column; gap:2px; }
  .cta-title { font-size:15px; font-weight:600; }
  .cta-sub { font-size:12px; opacity:0.85; }

  .recents-empty {
    display:flex; align-items:center; gap:10px;
    color:var(--text-muted); font-size:13px;
    padding:8px 4px;
  }

  .section { display:flex; flex-direction:column; gap:8px; }
  .section-head {
    display:flex; align-items:center; gap:8px;
    color:var(--text-faint); font-size:11px; font-weight:600;
    text-transform:uppercase; letter-spacing:0.6px;
    padding:0 4px;
  }
  .section-title { flex:1; }
  .link {
    background:transparent; border:0; cursor:pointer;
    color:var(--text-faint); font-size:11px;
    padding:0; text-decoration:underline;
  }
  .link:hover { color:var(--text-muted); }

  .recent-list { display:flex; flex-direction:column; gap:4px; }
  .recent {
    display:flex; align-items:center; gap:12px;
    padding:10px 12px;
    background:var(--bg-elevated);
    border:1px solid var(--border-subtle);
    border-radius:8px;
    cursor:pointer; text-align:left; color:var(--text); font:inherit;
    transition:background 80ms ease, border-color 80ms ease;
  }
  .recent:hover:not(:disabled) {
    border-color:var(--accent);
  }
  .recent:disabled { opacity:0.55; cursor:default; }
  .recent.stale { background:var(--bg-subtle); }
  .recent-info { flex:1; min-width:0; }
  .recent-title { display:flex; align-items:center; gap:8px; }
  .recent-name { font-weight:600; font-size:13px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .missing {
    color:var(--danger); font-size:9px; font-family:var(--font-mono);
    padding:0 4px; border:1px solid var(--border-subtle); border-radius:3px;
  }
  .recent-cfg { font-size:10px; color:var(--text-faint); font-family:var(--font-mono); }
  .recent-path { font-family:var(--font-mono); font-size:11px; color:var(--text-faint); margin-top:2px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .recent-time { font-size:10px; color:var(--text-faint); font-family:var(--font-mono); flex-shrink:0; }

  .footer { display:flex; justify-content:center; padding-top:6px; }
</style>
