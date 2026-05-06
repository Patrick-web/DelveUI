<script lang="ts">
  import { onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import * as SessionService from "../../bindings/github.com/jp/DelveUI/internal/services/sessionservice";
  import {
    workspace,
    sessions,
    openDebugFile,
    pickDebugFile,
    pickWorkspaceFolder,
    refreshWorkspace,
    refreshSessions,
    refreshTargets,
  } from "./store";
  import { debugFiles, loadDebugFiles, removeDebugFile, type DebugFileEntry } from "./settings-store";
  import Icon from "./Icon.svelte";

  type Tab = "folders" | "debugfiles";

  let paletteOpen = false;
  let tab: Tab = "folders";
  let filter = "";
  let pendingEntry: DebugFileEntry | null = null;
  let switching = false;
  let inputEl: HTMLInputElement | null = null;

  onMount(() => {
    loadDebugFiles();
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        if (pendingEntry) pendingEntry = null;
        else if (paletteOpen) paletteOpen = false;
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

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

  function fuzzy(q: string, s: string): boolean {
    if (!q) return true;
    const hay = s.toLowerCase();
    const needle = q.toLowerCase();
    let i = 0;
    for (const c of hay) {
      if (c === needle[i]) i++;
      if (i === needle.length) return true;
    }
    return false;
  }

  $: currentEntry = ($debugFiles ?? []).find(isCurrent) ?? null;
  $: currentLabel = labelFor(currentEntry, $workspace?.root ?? "");

  // Folders tab: entries that don't pin a specific debug file (the standard
  // .zed/.vscode/.delveui folder-walk path). Debug files tab: entries that
  // were registered by pointing directly at a launch.json/debug.json outside
  // those well-known locations (entry.launchFile is set).
  $: sortedEntries = ($debugFiles ?? []).slice().sort((a, b) => {
    if (!!a.stale !== !!b.stale) return a.stale ? 1 : -1;
    const at = new Date(a.lastUsed ?? a.addedAt).getTime() || 0;
    const bt = new Date(b.lastUsed ?? b.addedAt).getTime() || 0;
    return bt - at;
  });
  $: folderEntries = sortedEntries.filter((e) => !e.launchFile);
  $: debugFileEntries = sortedEntries.filter((e) => !!e.launchFile);
  $: tabEntries = tab === "folders" ? folderEntries : debugFileEntries;
  $: visibleEntries = filter
    ? tabEntries.filter((e) => fuzzy(filter, labelFor(e) + " " + e.path))
    : tabEntries;

  async function openPalette() {
    paletteOpen = true;
    await loadDebugFiles();
    await Promise.resolve();
    inputEl?.focus();
  }

  function pick(entry: DebugFileEntry) {
    if (entry.stale) return;
    if (isCurrent(entry)) {
      paletteOpen = false;
      return;
    }
    paletteOpen = false;
    pendingEntry = entry;
  }

  async function onRemove(e: Event, entry: DebugFileEntry) {
    e.stopPropagation();
    await removeDebugFile(entry.id);
  }

  function onRemoveKey(e: KeyboardEvent, entry: DebugFileEntry) {
    if (e.key === "Enter" || e.key === " ") onRemove(e, entry);
  }

  async function importFolder() {
    paletteOpen = false;
    await pickWorkspaceFolder();
    await refreshWorkspace();
    refreshTargets().catch(() => {});
  }

  async function importDebugFile() {
    paletteOpen = false;
    await pickDebugFile();
    await refreshWorkspace();
    refreshTargets().catch(() => {});
  }

  async function openHere() {
    if (!pendingEntry || switching) return;
    const target = pendingEntry;
    switching = true;
    try {
      const ids = Object.keys($sessions);
      await Promise.all(ids.map((id) => SessionService.Stop(id).catch(() => {})));
      await refreshSessions();
      await openDebugFile(target.path);
      await refreshWorkspace();
      refreshTargets().catch(() => {});
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

<button class="ps-pill" on:click={openPalette} title={currentEntry?.path ?? "Choose project"}>
  <Icon icon="solar:folder-bold" size={12} color="var(--accent)" />
  <span class="ps-label">{currentLabel || "No project"}</span>
  <Icon icon="solar:alt-arrow-down-linear" size={10} />
</button>

{#if paletteOpen}
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div
    class="pp-backdrop"
    role="presentation"
    on:click={() => (paletteOpen = false)}
    on:keydown={(e) => e.key === "Escape" && (paletteOpen = false)}
  ></div>
  <div class="pp-palette" role="dialog" aria-modal="true" on:click|stopPropagation on:keydown|stopPropagation>
    <div class="pp-tabs" role="tablist">
      <button
        class="pp-tab"
        class:active={tab === "folders"}
        role="tab"
        aria-selected={tab === "folders"}
        on:click={() => { tab = "folders"; filter = ""; }}
      >
        Folders <span class="pp-tab-count">{folderEntries.length}</span>
      </button>
      <button
        class="pp-tab"
        class:active={tab === "debugfiles"}
        role="tab"
        aria-selected={tab === "debugfiles"}
        on:click={() => { tab = "debugfiles"; filter = ""; }}
      >
        Debug files <span class="pp-tab-count">{debugFileEntries.length}</span>
      </button>
    </div>

    <div class="pp-search">
      <Icon icon="solar:magnifer-linear" size={12} color="var(--text-faint)" />
      <!-- svelte-ignore a11y-autofocus -->
      <input
        bind:this={inputEl}
        bind:value={filter}
        placeholder={tab === "folders" ? "Filter folders…" : "Filter debug files…"}
        spellcheck="false"
        autofocus
      />
    </div>

    <div class="pp-list">
      <button
        class="pp-import"
        on:click={tab === "folders" ? importFolder : importDebugFile}
      >
        <Icon
          icon={tab === "folders" ? "solar:folder-with-files-bold" : "solar:document-add-bold"}
          size={14}
          color="var(--accent)"
        />
        <span>{tab === "folders" ? "Import folder…" : "Import debug.json…"}</span>
      </button>

      {#if visibleEntries.length === 0}
        <div class="pp-empty">
          {#if tabEntries.length === 0}
            {tab === "folders"
              ? "No folders imported yet."
              : "No debug files imported yet."}
          {:else}
            No matches for &ldquo;{filter}&rdquo;.
          {/if}
        </div>
      {:else}
        {#each visibleEntries as e (e.id)}
          <button
            class="pp-item"
            class:active={isCurrent(e)}
            class:stale={e.stale}
            title={e.stale ? `Folder missing: ${e.path}` : e.path}
            on:click={() => pick(e)}
          >
            <Icon
              icon={tab === "folders" ? "solar:folder-open-bold" : "solar:document-text-bold"}
              size={14}
              color={e.stale ? "var(--text-faint)" : "var(--accent)"}
            />
            <div class="pp-item-body">
              <div class="pp-item-title">
                <span class="pp-item-name">{labelFor(e)}</span>
                {#if e.stale}
                  <span class="pp-stale-badge" title="Folder no longer exists">missing</span>
                {/if}
              </div>
              <div class="pp-item-path">{pathDisplay(e)}</div>
            </div>
            {#if isCurrent(e)}
              <Icon icon="solar:check-circle-bold" size={13} color="var(--success)" />
            {/if}
            <span
              class="pp-rm"
              role="button"
              tabindex="0"
              title="Remove from list"
              on:click={(ev) => onRemove(ev, e)}
              on:keydown={(ev) => onRemoveKey(ev, e)}
            >
              <Icon icon="solar:close-circle-linear" size={13} color="var(--text-faint)" />
            </span>
          </button>
        {/each}
      {/if}
    </div>
  </div>
{/if}

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
    --wails-draggable: no-drag;
  }
  .ps-pill:hover { background: rgba(255, 255, 255, 0.05); border-color: var(--border); }
  .ps-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 160px;
  }

  /* Centered command-palette-style popup */
  .pp-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    z-index: 850;
    /* The component is mounted inside the toolbar (which sets
       --wails-draggable: drag), so the modal would otherwise be draggable
       — and inputs inside a drag region don't accept focus on macOS. */
    --wails-draggable: no-drag;
  }
  .pp-palette {
    position: fixed;
    top: 18vh;
    left: 50%;
    transform: translateX(-50%);
    width: min(520px, calc(100vw - 48px));
    max-height: 64vh;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: 10px;
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.5);
    z-index: 851;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    --wails-draggable: no-drag;
  }
  .pp-tabs {
    display: flex;
    gap: 2px;
    padding: 6px;
    background: var(--bg-subtle);
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }
  .pp-tab {
    flex: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    height: 28px;
    padding: 0 10px;
    background: transparent;
    border: 0;
    border-radius: 5px;
    color: var(--text-faint);
    font: inherit;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }
  .pp-tab:hover { color: var(--text); background: rgba(255,255,255,0.04); }
  .pp-tab.active {
    color: var(--text);
    background: var(--bg-elevated);
    box-shadow: inset 0 0 0 1px var(--border);
  }
  .pp-tab-count {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-faint);
    font-weight: 400;
  }
  .pp-tab.active .pp-tab-count { color: var(--text-muted); }

  .pp-search {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }
  .pp-search input {
    flex: 1;
    background: transparent;
    border: 0;
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
    outline: none;
  }

  .pp-list {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 6px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .pp-import {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    background: transparent;
    border: 1px dashed var(--border);
    border-radius: 6px;
    color: var(--text);
    font: inherit;
    font-size: 12px;
    text-align: left;
    padding: 8px 10px;
    cursor: pointer;
    margin-bottom: 4px;
  }
  .pp-import:hover {
    border-color: var(--accent);
    color: var(--accent);
    background: var(--accent-subtle, rgba(77, 156, 255, 0.08));
  }

  .pp-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    background: transparent;
    border: 0;
    padding: 7px 10px;
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
    text-align: left;
    cursor: pointer;
    border-radius: 5px;
  }
  .pp-item:hover { background: rgba(255, 255, 255, 0.06); }
  .pp-item.active { background: rgba(94, 166, 255, 0.12); }
  .pp-item.stale { opacity: 0.55; cursor: default; }
  .pp-item.stale:hover { background: transparent; }
  .pp-item-body { flex: 1; min-width: 0; }
  .pp-item-title { display: flex; align-items: center; gap: 6px; }
  .pp-item-name {
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .pp-stale-badge {
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
  .pp-item-path {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .pp-rm {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    border-radius: 4px;
    cursor: pointer;
    flex-shrink: 0;
  }
  .pp-rm:hover { background: rgba(255, 255, 255, 0.08); }

  .pp-empty {
    padding: 20px 12px;
    text-align: center;
    color: var(--text-faint);
    font-size: 12px;
  }

  /* Confirm modal (existing) */
  .ps-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.45);
    z-index: 860;
    --wails-draggable: no-drag;
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
    z-index: 861;
    display: flex;
    flex-direction: column;
    gap: 14px;
    --wails-draggable: no-drag;
  }
  .ps-modal-head { display: flex; align-items: flex-start; gap: 12px; }
  .ps-modal h3 { margin: 0; font-size: 15px; font-weight: 700; color: var(--text); }
  .ps-modal-sub { margin: 2px 0 0; font-size: 13px; color: var(--text); }
  .ps-modal-path {
    margin: 1px 0 0;
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ps-modal-body { margin: 0; color: var(--text-muted); font-size: 12px; line-height: 1.5; }
  .ps-modal-actions { display: flex; gap: 8px; justify-content: flex-end; }
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
  .ps-btn.primary { background: var(--accent); border-color: var(--accent); color: #fff; }
  .ps-btn.primary:hover:not(:disabled) { background: #5ea6ff; }
  .ps-btn:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
