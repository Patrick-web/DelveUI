<script lang="ts">
  import { onMount } from "svelte";
  import Icon from "./Icon.svelte";
  import {
    debugFiles,
    loadDebugFiles,
    type DebugFileEntry,
  } from "./settings-store";
  import {
    workspace,
    refreshWorkspace,
    openDebugFile,
  } from "./store";

  export let open = false;

  let selected = 0;

  $: if (open) {
    loadDebugFiles();
    selected = 0;
  }

  $: activeFile = $debugFiles.find((f) => f.path === ($workspace?.root ?? ""));
  $: items = $debugFiles
    .slice()
    .sort((a, b) => {
      if (!!a.stale !== !!b.stale) return a.stale ? 1 : -1;
      const at = new Date(a.lastUsed ?? a.addedAt).getTime() || 0;
      const bt = new Date(b.lastUsed ?? b.addedAt).getTime() || 0;
      return bt - at;
    });

  function close() { open = false; }

  async function selectFile(f: DebugFileEntry) {
    if (f.stale) return;
    await openDebugFile(f.path);
    await refreshWorkspace();
    close();
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Escape") { close(); return; }
    if (e.key === "ArrowDown") { e.preventDefault(); selected = Math.min(items.length - 1, selected + 1); }
    if (e.key === "ArrowUp") { e.preventDefault(); selected = Math.max(0, selected - 1); }
    if (e.key === "Enter" && items[selected]) { selectFile(items[selected]); }
  }

  function shortPath(p: string) { return p.replace(/^\/Users\/[^/]+/, "~"); }
</script>

{#if open}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div class="backdrop" role="presentation" on:click={close}></div>
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div class="picker" role="dialog" aria-modal="true" on:keydown={onKey}>
    <div class="header">
      <Icon icon="solar:folder-open-bold" size={15} color="var(--accent)" />
      <span class="title">Debug Configurations</span>
      <button class="btn icon" on:click={close} title="Close">
        <Icon icon="solar:close-circle-linear" size={14} />
      </button>
    </div>

    <div class="list">
      {#if items.length === 0}
        <div class="empty">
          <span>No folders imported yet.</span>
          <span class="sub">Open a folder to auto-detect debug configurations.</span>
        </div>
      {/if}
      {#each items as f, i}
        <button
          class="item"
          class:sel={i === selected}
          class:active={activeFile?.id === f.id}
          class:stale={f.stale}
          on:click={() => selectFile(f)}
          on:mouseenter={() => (selected = i)}
        >
          <div class="item-left">
            <Icon
              icon={activeFile?.id === f.id ? "solar:check-circle-bold" : "solar:folder-open-bold"}
              size={12}
              color={f.stale ? "var(--text-faint)" : (activeFile?.id === f.id ? "var(--success)" : "var(--accent)")}
            />
            <div class="item-info">
              <span class="item-label">
                {f.label}
                {#if f.stale}<span class="missing">missing</span>{/if}
              </span>
              <span class="item-path">{shortPath(f.path)}</span>
            </div>
          </div>
          <span class="item-count">{f.configs?.length ?? 0} configs</span>
        </button>
      {/each}
    </div>
  </div>
{/if}

<style>
  .backdrop { position:fixed; inset:0; background:rgba(0,0,0,0.35); z-index:850; }
  .picker {
    position:fixed; top:20vh; left:50%; transform:translateX(-50%);
    width:480px; max-height:60vh;
    background:var(--bg-elevated); border:1px solid var(--border);
    border-radius:var(--radius-md); box-shadow:0 24px 64px rgba(0,0,0,0.5);
    z-index:851; display:flex; flex-direction:column; overflow:hidden;
  }
  .header {
    display:flex; align-items:center; gap:var(--space-2);
    padding:var(--space-2) var(--space-3);
    border-bottom:1px solid var(--border);
  }
  .title { flex:1; font-size:var(--text-sm); font-weight:600; color:var(--text); }

  .list { flex:1; overflow:auto; padding:var(--space-1) 0; }
  .empty { padding:var(--space-6) var(--space-4); text-align:center; color:var(--text-muted); font-size:var(--text-sm); display:flex; flex-direction:column; gap:var(--space-1); }
  .sub { font-size:var(--text-xs); color:var(--text-faint); }

  .item {
    display:flex; align-items:center; justify-content:space-between;
    width:100%; background:transparent; border:0;
    padding:var(--space-2) var(--space-3); cursor:pointer;
    text-align:left; gap:var(--space-2);
  }
  .item:hover, .item.sel { background:var(--accent-subtle); }
  .item.active { border-left:2px solid var(--accent); }
  .item.stale { opacity:0.55; cursor:default; }
  .item.stale:hover { background:transparent; }
  .missing { color:var(--danger); font-size:9px; font-family:var(--font-mono); margin-left:6px; padding:0 4px; border:1px solid var(--border-subtle); border-radius:3px; }
  .item-left { display:flex; align-items:center; gap:var(--space-2); flex:1; min-width:0; }
  .item-info { display:flex; flex-direction:column; min-width:0; }
  .item-label { font-size:var(--text-sm); font-weight:500; color:var(--text); }
  .item-path { font-size:var(--text-xs); color:var(--text-faint); font-family:var(--font-mono); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .item-count { font-size:var(--text-xs); color:var(--text-faint); white-space:nowrap; }
</style>
