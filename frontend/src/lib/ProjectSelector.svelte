<script lang="ts">
  import { onMount } from "svelte";
  import { debugFiles, loadDebugFiles, type DebugFileEntry } from "./settings-store";
  import { openDebugFile, pickWorkspaceFolder, refreshWorkspace, refreshTargets } from "./store";
  import Icon from "./Icon.svelte";

  let loading = true;

  onMount(async () => {
    await loadDebugFiles();
    loading = false;
  });

  function short(p: string): string {
    return (p ?? "").replace(/^\/Users\/[^/]+/, "~");
  }

  function dirOf(p: string): string {
    const s = short(p);
    const i = s.lastIndexOf("/");
    return i >= 0 ? s.slice(0, i) : s;
  }

  async function openFile(entry: DebugFileEntry) {
    await openDebugFile(entry.path);
    await refreshWorkspace();
    refreshTargets().catch(() => {});
  }

  async function pickFolder() {
    await pickWorkspaceFolder();
    await refreshWorkspace();
    refreshTargets().catch(() => {});
  }

  $: entries = $debugFiles ?? [];
  $: sorted = [...entries].sort((a, b) => {
    if (!!a.stale !== !!b.stale) return a.stale ? 1 : -1;
    const at = new Date(a.lastUsed ?? a.addedAt).getTime() || 0;
    const bt = new Date(b.lastUsed ?? b.addedAt).getTime() || 0;
    return bt - at;
  });
</script>

<div class="selector">
  <div class="inner">
    <div class="head">
      <Icon icon="solar:folder-bold-duotone" size={24} color="var(--accent)" />
      <div>
        <h1>Choose a project</h1>
        <p class="subtitle">
          Open a folder to auto-detect Go targets and debug configurations.
        </p>
      </div>
    </div>

    {#if loading}
      <div class="empty">
        <div class="spinner" aria-label="Loading"></div>
      </div>
    {:else if sorted.length === 0}
      <div class="empty">
        <Icon icon="solar:inbox-bold" size={24} color="var(--text-faint)" />
        <span>No projects yet.</span>
        <span class="sub">Open a folder to get started — debug configs are auto-detected.</span>
      </div>
    {:else}
      <ul class="list">
        {#each sorted as e (e.id)}
          <li>
            <button class="card" on:click={() => openFile(e)} title={e.path}>
              <div class="card-icon">
                <Icon icon="solar:folder-bold" size={18} color="var(--accent)" />
              </div>
              <div class="card-body">
                <div class="card-title-row">
                  <span class="card-title">{e.label || dirOf(e.path).split("/").pop()}</span>
                  {#if e.stale}
                    <span class="badge stale" title="Folder no longer exists">missing</span>
                  {/if}
                  {#if e.configs && e.configs.length}
                    <span class="cfg-count">{e.configs.length} cfg{e.configs.length !== 1 ? "s" : ""}</span>
                  {/if}
                </div>
                <div class="card-path">{short(e.path)}</div>
              </div>
              <div class="card-cta">
                <Icon icon="solar:alt-arrow-right-linear" size={14} color="var(--text-faint)" />
              </div>
            </button>
          </li>
        {/each}
      </ul>
    {/if}

    <div class="actions">
      <button class="pill primary" on:click={pickFolder}>
        <Icon icon="solar:folder-with-files-bold" size={13} />
        Open folder…
      </button>
    </div>
  </div>
</div>

<style>
  .selector {
    flex: 1;
    min-height: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 40px 24px;
    background: var(--bg);
    overflow: auto;
  }
  .inner {
    width: 100%;
    max-width: 560px;
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .head {
    display: flex;
    align-items: center;
    gap: 14px;
  }
  h1 {
    margin: 0;
    font-size: 18px;
    font-weight: 700;
    color: var(--text);
  }
  .subtitle {
    margin: 2px 0 0;
    color: var(--text-muted);
    font-size: 12px;
  }
  .subtitle code {
    background: var(--bg-subtle);
    padding: 1px 5px;
    border-radius: 3px;
    font-family: var(--font-mono);
    font-size: 11px;
  }

  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 32px 12px;
    color: var(--text-muted);
    font-size: 13px;
    text-align: center;
    border: 1px dashed var(--border-subtle);
    border-radius: var(--radius-md);
  }
  .empty .sub { color: var(--text-faint); font-size: 11px; }
  .empty code {
    font-family: var(--font-mono);
    background: var(--bg-subtle);
    padding: 1px 5px;
    border-radius: 3px;
    font-size: 11px;
  }

  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-height: 50vh;
    overflow: auto;
  }
  .card {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    padding: 10px 12px;
    cursor: pointer;
    color: var(--text);
    text-align: left;
    font: inherit;
    transition: background 80ms ease, border-color 80ms ease;
  }
  .card:hover {
    border-color: var(--accent);
    background: var(--bg-elevated);
  }
  .card-icon { flex-shrink: 0; }
  .card-body { flex: 1; min-width: 0; }
  .card-title-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 2px;
  }
  .card-title {
    font-size: 13px;
    font-weight: 600;
    color: var(--text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .card-path {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .card-cta { flex-shrink: 0; }

  .badge {
    font-size: 9px;
    font-weight: 700;
    color: #fff;
    background: var(--accent);
    padding: 1px 6px;
    border-radius: 8px;
    letter-spacing: 0.4px;
    text-transform: uppercase;
  }
  .badge.stale {
    background: var(--danger);
  }
  .cfg-count {
    font-size: 10px;
    color: var(--text-faint);
    font-family: var(--font-mono);
  }

  .actions {
    display: flex;
    gap: 8px;
    justify-content: center;
    padding-top: 4px;
  }
  .pill {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 28px;
    padding: 0 12px;
    background: transparent;
    border: 0;
    border-radius: 6px;
    color: var(--text);
    font: inherit;
    font-size: 12px;
    cursor: pointer;
    transition: background 80ms ease;
  }
  .pill:hover { background: rgba(255, 255, 255, 0.06); }
  .pill.primary {
    background: var(--accent);
    color: #fff;
    padding: 0 14px;
  }
  .pill.primary:hover { background: #5ea6ff; }

  .spinner {
    width: 16px; height: 16px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: ps-spin 0.8s linear infinite;
  }
  @keyframes ps-spin { to { transform: rotate(360deg); } }
</style>
