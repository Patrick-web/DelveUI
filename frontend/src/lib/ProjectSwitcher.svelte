<script lang="ts">
  import { onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import * as SessionService from "../../bindings/github.com/jp/DelveUI/internal/services/sessionservice";
  import { workspace, sessions, openDebugFile, refreshWorkspace, refreshSessions } from "./store";
  import { debugFiles, loadDebugFiles, removeDebugFile, type DebugFileEntry } from "./settings-store";
  import Icon from "./Icon.svelte";

  let dropdownOpen = false;
  let pendingEntry: DebugFileEntry | null = null;
  let switching = false;
  let rootEl: HTMLDivElement;

  onMount(() => {
    loadDebugFiles();
    function onDocClick(e: MouseEvent) {
      if (!rootEl) return;
      if (!rootEl.contains(e.target as Node)) dropdownOpen = false;
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        if (pendingEntry) pendingEntry = null;
        else dropdownOpen = false;
      }
    }
    document.addEventListener("mousedown", onDocClick);
    window.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDocClick);
      window.removeEventListener("keydown", onKey);
    };
  });

  // Every project entry is now a folder; we still derive the label from the
  // backend-supplied label when present so legacy migrated entries keep their
  // user-facing name.
  function pathDisplay(entry: DebugFileEntry | null | undefined): string {
    const p = entry?.path ?? "";
    return p ? p.replace(/^\/Users\/[^/]+/, "~") : "";
  }

  function labelFor(entry: DebugFileEntry | null | undefined, fallbackPath = ""): string {
    if (entry?.label) return entry.label;
    const path = entry?.path ?? fallbackPath;
    if (!path) return "";
    const parts = path.split("/").filter(Boolean);
    return parts[parts.length - 1] ?? path;
  }

  function isCurrent(entry: DebugFileEntry): boolean {
    return entry.path === ($workspace?.root ?? "");
  }

  $: currentEntry = ($debugFiles ?? []).find(isCurrent) ?? null;
  $: currentLabel = labelFor(currentEntry, $workspace?.root ?? "");
  // MRU sort: most recent first, with stale entries pushed to the bottom so
  // healthy projects always lead the list.
  $: entries = ($debugFiles ?? []).slice().sort((a, b) => {
    if (!!a.stale !== !!b.stale) return a.stale ? 1 : -1;
    const at = new Date(a.lastUsed ?? a.addedAt).getTime() || 0;
    const bt = new Date(b.lastUsed ?? b.addedAt).getTime() || 0;
    return bt - at;
  });

  async function toggle() {
    if (!dropdownOpen) await loadDebugFiles();
    dropdownOpen = !dropdownOpen;
  }

  function pick(entry: DebugFileEntry) {
    dropdownOpen = false;
    if (entry.stale) return; // can't open a missing folder
    if (isCurrent(entry)) return;
    pendingEntry = entry;
  }

  async function onRemove(e: Event, entry: DebugFileEntry) {
    e.stopPropagation();
    await removeDebugFile(entry.id);
  }

  function onRemoveKey(e: KeyboardEvent, entry: DebugFileEntry) {
    if (e.key === "Enter" || e.key === " ") onRemove(e, entry);
  }

  async function openHere() {
    if (!pendingEntry || switching) return;
    const target = pendingEntry;
    switching = true;
    try {
      // Stop every running session before swapping projects.
      const ids = Object.keys($sessions);
      await Promise.all(ids.map((id) => SessionService.Stop(id).catch(() => {})));
      await refreshSessions();
      await openDebugFile(target.path);
      await refreshWorkspace();
    } finally {
      switching = false;
      pendingEntry = null;
    }
  }

  async function openInNew() {
    if (!pendingEntry) return;
    const target = pendingEntry;
    pendingEntry = null;
    try {
      await Events.Emit("project:open-new-window", target.path);
    } catch (e) {
      console.error("open new window failed:", e);
    }
  }
</script>

<div class="ps-root" bind:this={rootEl}>
  <button class="ps-pill" on:click={toggle} title={currentEntry?.path ?? "Choose project"}>
    <Icon icon="solar:folder-bold" size={12} color="var(--accent)" />
    <span class="ps-label">{currentLabel || "No project"}</span>
    <Icon icon="solar:alt-arrow-down-linear" size={10} />
  </button>

  {#if dropdownOpen}
    <div class="ps-dd" role="menu">
      {#if entries.length === 0}
        <div class="ps-empty">No projects registered yet.</div>
      {/if}
      {#each entries as e (e.id)}
        <button
          class="ps-item"
          class:active={isCurrent(e)}
          class:stale={e.stale}
          title={e.stale ? `Folder missing: ${e.path}` : e.path}
          on:click={() => pick(e)}
        >
          <Icon
            icon="solar:folder-open-bold"
            size={12}
            color={e.stale ? "var(--text-faint)" : "var(--accent)"}
          />
          <div class="ps-item-body">
            <div class="ps-item-title">
              <span class="ps-item-name">{labelFor(e)}</span>
              {#if e.stale}
                <span class="ps-stale-badge" title="Folder no longer exists">missing</span>
              {/if}
            </div>
            <div class="ps-item-path">{pathDisplay(e)}</div>
          </div>
          {#if isCurrent(e)}
            <Icon icon="solar:check-circle-bold" size={12} color="var(--success)" />
          {/if}
          <span
            class="ps-rm"
            role="button"
            tabindex="0"
            title="Remove from list"
            on:click={(ev) => onRemove(ev, e)}
            on:keydown={(ev) => onRemoveKey(ev, e)}
          >
            <Icon icon="solar:close-circle-linear" size={12} color="var(--text-faint)" />
          </span>
        </button>
      {/each}
    </div>
  {/if}
</div>

{#if pendingEntry}
  <div class="ps-backdrop" on:click={() => (pendingEntry = null)}></div>
  <div class="ps-modal" role="dialog" aria-modal="true">
    <div class="ps-modal-head">
      <Icon icon="solar:folder-bold-duotone" size={20} color="var(--accent)" />
      <div>
        <h3>Switch project?</h3>
        <p class="ps-modal-sub">{labelFor(pendingEntry)}</p>
        <p class="ps-modal-path">{pathDisplay(pendingEntry)}</p>
      </div>
    </div>
    <p class="ps-modal-body">
      {#if Object.keys($sessions).length === 0}
        No debug sessions are running. Choose where to open the project.
      {:else if Object.keys($sessions).length === 1}
        Opening in this window will stop the running debug session.
      {:else}
        Opening in this window will stop the {Object.keys($sessions).length} running debug sessions.
      {/if}
    </p>
    <div class="ps-modal-actions">
      <button class="ps-btn" on:click={() => (pendingEntry = null)} disabled={switching}>Cancel</button>
      <button class="ps-btn" on:click={openInNew} disabled={switching}>Open in new window</button>
      <button class="ps-btn primary" on:click={openHere} disabled={switching}>
        {switching ? "Opening…" : "Open in this window"}
      </button>
    </div>
  </div>
{/if}

<style>
  .ps-root {
    position: relative;
    --wails-draggable: no-drag;
  }
  .ps-pill {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 26px;
    padding: 0 10px;
    background: transparent;
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
    cursor: pointer;
    max-width: 220px;
  }
  .ps-pill:hover { background: rgba(255, 255, 255, 0.05); border-color: var(--border); }
  .ps-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 160px;
  }

  .ps-dd {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: 8px;
    min-width: 320px;
    z-index: 100;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.45);
    padding: 4px;
    max-height: 360px;
    overflow: auto;
  }
  .ps-empty {
    padding: 12px;
    color: var(--text-faint);
    font-size: var(--text-sm);
    text-align: center;
  }
  .ps-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    background: transparent;
    border: 0;
    padding: 8px 10px;
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
    text-align: left;
    cursor: pointer;
    border-radius: 5px;
  }
  .ps-item:hover { background: rgba(255, 255, 255, 0.06); }
  .ps-item.active { background: rgba(94, 166, 255, 0.12); }
  .ps-item-body { flex: 1; min-width: 0; }
  .ps-item-title {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .ps-item-name {
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ps-stale-badge {
    font-family: var(--font-mono);
    font-size: 9px;
    color: var(--danger);
    background: var(--bg-subtle);
    border: 1px solid var(--border-subtle);
    border-radius: 3px;
    padding: 0 4px;
    margin-left: 6px;
    flex-shrink: 0;
  }
  .ps-item.stale {
    opacity: 0.55;
    cursor: default;
  }
  .ps-item.stale:hover {
    background: transparent;
  }
  .ps-rm {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    border-radius: 4px;
    cursor: pointer;
    flex-shrink: 0;
  }
  .ps-rm:hover {
    background: rgba(255, 255, 255, 0.08);
  }
  .ps-item-path {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ps-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.45);
    z-index: 200;
  }
  .ps-modal {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: min(460px, calc(100vw - 48px));
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: 10px;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.55);
    padding: 20px;
    z-index: 201;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .ps-modal-head {
    display: flex;
    align-items: flex-start;
    gap: 12px;
  }
  .ps-modal h3 {
    margin: 0;
    font-size: 15px;
    font-weight: 700;
    color: var(--text);
  }
  .ps-modal-sub {
    margin: 2px 0 0;
    font-size: 13px;
    color: var(--text);
  }
  .ps-modal-path {
    margin: 1px 0 0;
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ps-modal-body {
    margin: 0;
    color: var(--text-muted);
    font-size: 12px;
    line-height: 1.5;
  }
  .ps-modal-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
  }
  .ps-btn {
    height: 30px;
    padding: 0 14px;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
    cursor: pointer;
  }
  .ps-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.06); }
  .ps-btn.primary {
    background: var(--accent);
    border-color: var(--accent);
    color: #fff;
  }
  .ps-btn.primary:hover:not(:disabled) { background: #5ea6ff; }
  .ps-btn:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
