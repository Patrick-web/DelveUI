<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import Icon from "./Icon.svelte";
  import type { GoSymbol } from "./go-symbols";

  export let open = false;
  export let symbols: GoSymbol[] = [];

  const dispatch = createEventDispatcher<{ pick: GoSymbol }>();

  let input = "";
  let selected = 0;

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

  $: filtered = symbols.filter((s) => fuzzy(input, s.name));
  $: if (input !== undefined) selected = 0;
  $: if (!open) { input = ""; selected = 0; }

  function close() { open = false; }

  function pick(sym: GoSymbol) {
    dispatch("pick", sym);
    close();
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Escape") close();
    else if (e.key === "Enter" && filtered[selected]) pick(filtered[selected]);
    else if (e.key === "ArrowDown") {
      e.preventDefault();
      selected = Math.min(filtered.length - 1, selected + 1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      selected = Math.max(0, selected - 1);
    }
  }

  function kindIcon(k: GoSymbol["kind"]): string {
    if (k === "func") return "solar:code-square-bold";
    if (k === "method") return "solar:code-bold";
    return "solar:box-minimalistic-bold";
  }
  function kindColor(k: GoSymbol["kind"]): string {
    if (k === "func") return "var(--info)";
    if (k === "method") return "var(--accent)";
    return "var(--warning)";
  }
  function kindLabel(k: GoSymbol["kind"]): string {
    if (k === "func") return "func";
    if (k === "method") return "method";
    return "type";
  }
</script>

{#if open}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
  <div class="backdrop" role="presentation" on:click={close}></div>
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div class="modal" role="dialog" aria-modal="true" on:keydown={onKey}>
    <div class="search-row">
      <Icon icon="solar:magnifer-linear" size={14} color="var(--text-faint)" />
      <!-- svelte-ignore a11y-autofocus -->
      <input class="search" bind:value={input} placeholder="Go to symbol…" autofocus />
      <span class="count">{filtered.length} / {symbols.length}</span>
    </div>
    <div class="list">
      {#each filtered as s, i}
        <button
          class="item"
          class:sel={i === selected}
          on:click={() => pick(s)}
          on:mouseenter={() => (selected = i)}
        >
          <Icon icon={kindIcon(s.kind)} size={12} color={kindColor(s.kind)} />
          <span class="name">{s.name}</span>
          <span class="kind">{kindLabel(s.kind)}</span>
          <span class="line">:{s.line}</span>
        </button>
      {/each}
      {#if filtered.length === 0}
        <div class="empty">{symbols.length === 0 ? "No symbols in this file" : "No matches"}</div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .backdrop { position:fixed; inset:0; background:rgba(0,0,0,0.35); z-index:900; }
  .modal {
    position:fixed; top:18vh; left:50%; transform:translateX(-50%);
    width:520px; max-height:55vh; background:var(--bg-elevated);
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
  .count {
    font-size:var(--text-xs); color:var(--text-faint);
    font-family:var(--font-mono);
  }
  .list { flex:1; overflow:auto; padding:var(--space-1) 0; }
  .item {
    display:flex; align-items:center; gap:var(--space-2); width:100%;
    background:transparent; border:0; padding:6px var(--space-3);
    font-size:var(--text-sm); color:var(--text); cursor:pointer;
    text-align:left; font-family:var(--font-mono);
  }
  .item.sel { background:var(--accent-subtle); }
  .name { flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .kind {
    font-size:10px; color:var(--text-faint); text-transform:uppercase;
    letter-spacing:0.5px; font-family:var(--font-ui);
  }
  .line { color:var(--text-faint); font-size:var(--text-xs); }
  .empty { padding:var(--space-3); color:var(--text-faint); text-align:center; font-size:var(--text-sm); }
</style>
