<script lang="ts">
  import { activeSessionId, sessionState, clearSessionOutput } from "./store";
  import TerminalPane from "./TerminalPane.svelte";
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";
  import { showInfo } from "./toast";

  $: output = $activeSessionId
    ? ($sessionState[$activeSessionId]?.output ?? [])
    : [];

  // Search
  let searchOpen = false;
  let searchQuery = "";
  let matchCount = 0;
  let currentMatch = 0;

  function toggleSearch() {
    searchOpen = !searchOpen;
    if (!searchOpen) { searchQuery = ""; matchCount = 0; currentMatch = 0; }
  }

  // Filter
  type Filter = "all" | "stdout" | "stderr" | "dlv" | "console";
  let filter: Filter = "all";
  const filterFn = (cat: string): boolean => {
    if (filter === "all") return true;
    if (filter === "stdout") return cat === "stdout";
    if (filter === "stderr") return cat === "stderr";
    if (filter === "dlv") return cat === "dlv-stdout" || cat === "dlv-stderr";
    if (filter === "console") return cat === "console" || cat === "important";
    return true;
  };

  $: filteredOutput = output.filter((l) => filterFn(l.cat));

  async function copy() {
    const text = filteredOutput.map((l) => l.text).join("");
    await navigator.clipboard.writeText(text);
    showInfo("Copied", `${filteredOutput.length} lines copied`);
  }

  function clear() {
    if ($activeSessionId) clearSessionOutput($activeSessionId);
  }

  function onKey(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === "f") {
      e.preventDefault();
      toggleSearch();
    }
  }
</script>

<svelte:window on:keydown={onKey} />

<PanelHeader title="Terminal">
  <select class="filter-select" bind:value={filter}>
    <option value="all">All</option>
    <option value="stdout">stdout</option>
    <option value="stderr">stderr</option>
    <option value="dlv">dlv</option>
    <option value="console">console</option>
  </select>
  <span class="dim">{filteredOutput.length}</span>
  <button class="btn icon" title="Search (⌘F)" on:click={toggleSearch}>
    <Icon icon="solar:magnifer-linear" size={13} />
  </button>
  <button class="btn icon" title="Copy" on:click={copy}>
    <Icon icon="solar:copy-linear" size={13} />
  </button>
  <button class="btn icon" title="Clear" on:click={clear}>
    <Icon icon="solar:eraser-linear" size={13} />
  </button>
</PanelHeader>

{#if searchOpen}
  <div class="search-bar">
    <Icon icon="solar:magnifer-linear" size={12} color="var(--text-faint)" />
    <!-- svelte-ignore a11y-autofocus -->
    <input
      class="search-input"
      bind:value={searchQuery}
      placeholder="Search output…"
      autofocus
      on:keydown={(e) => e.key === "Escape" && toggleSearch()}
    />
    {#if searchQuery}
      <span class="search-count">{matchCount} matches</span>
    {/if}
  </div>
{/if}

<div class="body">
  <TerminalPane lines={filteredOutput} {searchQuery} bind:matchCount />
</div>

<style>
  .body { flex:1; display:flex; flex-direction:column; min-height:0; }
  .dim { color:var(--text-faint); font-size:var(--text-xs); font-family:var(--font-mono); }
  .filter-select {
    background:var(--bg-subtle); border:1px solid var(--border-subtle);
    color:var(--text-muted); font-size:var(--text-xs); font-family:var(--font-mono);
    padding:1px 4px; border-radius:var(--radius-sm); cursor:pointer; outline:none;
  }
  .search-bar {
    display:flex; align-items:center; gap:var(--space-2);
    padding:var(--space-1) var(--space-3);
    border-bottom:1px solid var(--border-subtle); background:var(--bg);
  }
  .search-input {
    flex:1; background:transparent; border:0; color:var(--text);
    font-family:var(--font-mono); font-size:var(--text-sm); outline:none;
  }
  .search-count { font-size:var(--text-xs); color:var(--text-faint); font-family:var(--font-mono); white-space:nowrap; }
</style>
