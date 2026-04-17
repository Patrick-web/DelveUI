<script lang="ts">
  import { workspace, manualSourcePath } from "./store";
  import { setActivePanel } from "./panels/layout";
  import Icon from "./Icon.svelte";
  import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";

  export let open = false;

  let input = "";
  let selected = 0;
  let files: string[] = [];
  let loaded = false;
  let root = "";

  $: if (open && !loaded) loadFiles();

  async function loadFiles() {
    root = $workspace?.root ?? "";
    if (!root) return;
    try {
      files = (await FileService.ListGoFiles(root)) as string[] ?? [];
      loaded = true;
    } catch { files = []; }
  }

  function fuzzy(q: string, s: string): boolean {
    if (!q) return true;
    const hay = s.toLowerCase();
    const needle = q.toLowerCase();
    let i = 0;
    for (const c of hay) { if (c === needle[i]) i++; if (i === needle.length) return true; }
    return false;
  }

  $: filtered = files.filter((f) => fuzzy(input, f)).slice(0, 50);
  $: if (input !== undefined) selected = 0;

  function close() { open = false; input = ""; selected = 0; }

  function pick(file: string) {
    manualSourcePath.set(root + "/" + file);
    setActivePanel("right", "source");
    close();
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Escape") close();
    else if (e.key === "Enter" && filtered[selected]) pick(filtered[selected]);
    else if (e.key === "ArrowDown") { e.preventDefault(); selected = Math.min(filtered.length - 1, selected + 1); }
    else if (e.key === "ArrowUp") { e.preventDefault(); selected = Math.max(0, selected - 1); }
  }

  function dirPart(f: string) {
    const i = f.lastIndexOf("/");
    return i >= 0 ? f.substring(0, i + 1) : "";
  }
  function namePart(f: string) {
    const i = f.lastIndexOf("/");
    return i >= 0 ? f.substring(i + 1) : f;
  }
</script>

{#if open}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div class="backdrop" role="presentation" on:click={close}></div>
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div class="modal" role="dialog" aria-modal="true" on:keydown={onKey}>
    <div class="search-row">
      <Icon icon="solar:magnifer-linear" size={14} color="var(--text-faint)" />
      <!-- svelte-ignore a11y-autofocus -->
      <input class="search" bind:value={input} placeholder="Open file…" autofocus />
      <span class="count">{filtered.length} / {files.length}</span>
    </div>
    <div class="list">
      {#each filtered as f, i}
        <button class="item" class:sel={i === selected} on:click={() => pick(f)} on:mouseenter={() => (selected = i)}>
          <Icon icon="solar:code-bold" size={12} color="var(--info)" />
          <span class="dir">{dirPart(f)}</span><span class="file">{namePart(f)}</span>
        </button>
      {/each}
      {#if filtered.length === 0}
        <div class="empty">{files.length === 0 ? "No Go files found" : "No matches"}</div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .backdrop { position:fixed; inset:0; background:rgba(0,0,0,0.35); z-index:900; }
  .modal {
    position:fixed; top:18vh; left:50%; transform:translateX(-50%);
    width:540px; max-height:55vh; background:var(--bg-elevated);
    border:1px solid var(--border); border-radius:var(--radius-md);
    box-shadow:0 24px 64px rgba(0,0,0,0.5); z-index:901;
    display:flex; flex-direction:column; overflow:hidden;
  }
  .search-row {
    display:flex; align-items:center; gap:var(--space-2);
    padding:0 var(--space-3); height:40px;
    border-bottom:1px solid var(--border);
  }
  .search {
    flex:1; background:transparent; border:0; color:var(--text);
    font-family:var(--font-mono); font-size:var(--text-md); outline:none;
  }
  .count { font-size:var(--text-xs); color:var(--text-faint); font-family:var(--font-mono); }
  .list { flex:1; overflow:auto; padding:var(--space-1) 0; }
  .item {
    display:flex; align-items:center; gap:var(--space-2); width:100%;
    background:transparent; border:0; padding:5px var(--space-3);
    font-size:var(--text-sm); color:var(--text); cursor:pointer;
    text-align:left; font-family:var(--font-mono);
  }
  .item.sel { background:var(--accent-subtle); }
  .dir { color:var(--text-faint); }
  .file { color:var(--text); }
  .empty { padding:var(--space-3); color:var(--text-faint); text-align:center; font-size:var(--text-sm); }
</style>
