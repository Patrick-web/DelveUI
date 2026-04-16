<script lang="ts">
  import { onMount } from "svelte";
  import Icon from "./Icon.svelte";
  import {
    themeList,
    currentThemeName,
    refreshThemeList,
    setTheme,
    loadTheme,
    removeTheme,
    type ThemeMeta,
  } from "./theme-engine";
  import * as ThemeService from "../../bindings/github.com/jp/DelveUI/internal/themes/service";

  export let open = false;
  let filter = "";
  let previewName: string | null = null;
  let savedName = "";

  $: if (open) {
    savedName = $currentThemeName;
    refreshThemeList();
  }

  $: filtered = ($themeList ?? []).filter(
    (t) => !filter || t.name.toLowerCase().includes(filter.toLowerCase()),
  );

  $: darkThemes = filtered.filter((t) => t.appearance === "dark");
  $: lightThemes = filtered.filter((t) => t.appearance === "light");

  function close() {
    if (previewName && previewName !== $currentThemeName) {
      loadTheme(savedName);
    }
    previewName = null;
    open = false;
    filter = "";
  }

  let justSelected = false;

  function preview(t: ThemeMeta) {
    if (justSelected) return;
    previewName = t.name;
    loadTheme(t.name);
  }

  function revert() {
    if (justSelected) return;
    if (previewName) {
      loadTheme(savedName);
      previewName = null;
    }
  }

  function select(t: ThemeMeta) {
    justSelected = true;
    previewName = null;
    setTheme(t.name);
    open = false;
    filter = "";
    setTimeout(() => (justSelected = false), 100);
  }

  async function installFromFile() {
    try {
      const meta = (await ThemeService.ImportFile("")) as any;
      if (meta?.name) await refreshThemeList();
    } catch (e) {
      console.error(e);
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Escape") close();
  }
</script>

{#if open}
  <div
    class="backdrop"
    role="presentation"
    on:click={close}
    on:keydown={onKey}
  >
    <div class="picker" role="dialog" aria-modal="true" tabindex="-1" on:click|stopPropagation on:keydown|stopPropagation>
      <header>
        <Icon icon="solar:palette-bold" size={16} color="var(--accent)" />
        <span class="title">Themes</span>
        <input
          class="tx search"
          placeholder="Filter themes…"
          bind:value={filter}
          on:keydown={onKey}
        />
        <button class="btn outlined" on:click={installFromFile}>
          <Icon icon="solar:upload-minimalistic-bold" size={13} /> Install
        </button>
        <button class="btn icon" on:click={close} title="Close">
          <Icon icon="solar:close-circle-linear" size={14} />
        </button>
      </header>

      <div class="body">
        {#if darkThemes.length}
          <div class="section-label">Dark</div>
          <div class="grid">
            {#each darkThemes as t}
              <button
                class="card"
                class:active={t.name === $currentThemeName && !previewName}
                class:previewing={t.name === previewName}
                on:mouseenter={() => preview(t)}
                on:mouseleave={revert}
                on:click={() => select(t)}
              >
                <div class="card-name">{t.name}</div>
                <div class="card-meta">
                  {t.author}
                  {#if t.bundled}
                    <Icon icon="solar:lock-keyhole-linear" size={10} />
                  {:else}
                    <button class="rm" title="Remove" on:click|stopPropagation={() => removeTheme(t.name)}>
                      <Icon icon="solar:trash-bin-minimalistic-linear" size={11} />
                    </button>
                  {/if}
                </div>
              </button>
            {/each}
          </div>
        {/if}

        {#if lightThemes.length}
          <div class="section-label">Light</div>
          <div class="grid">
            {#each lightThemes as t}
              <button
                class="card"
                class:active={t.name === $currentThemeName && !previewName}
                class:previewing={t.name === previewName}
                on:mouseenter={() => preview(t)}
                on:mouseleave={revert}
                on:click={() => select(t)}
              >
                <div class="card-name">{t.name}</div>
                <div class="card-meta">
                  {t.author}
                  {#if t.bundled}
                    <Icon icon="solar:lock-keyhole-linear" size={10} />
                  {:else}
                    <button class="rm" title="Remove" on:click|stopPropagation={() => removeTheme(t.name)}>
                      <Icon icon="solar:trash-bin-minimalistic-linear" size={11} />
                    </button>
                  {/if}
                </div>
              </button>
            {/each}
          </div>
        {/if}

        {#if !filtered.length}
          <div class="empty">No themes match "{filter}"</div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .backdrop { position:fixed; inset:0; background:rgba(0,0,0,0.4); z-index:850; display:flex; align-items:flex-start; justify-content:center; padding-top:10vh; }
  .picker { background:var(--bg-elevated); border:1px solid var(--border); border-radius:var(--radius-md); width:620px; max-height:70vh; display:flex; flex-direction:column; overflow:hidden; box-shadow:0 24px 64px rgba(0,0,0,0.5); }
  header { display:flex; align-items:center; gap:var(--space-2); padding:var(--space-2) var(--space-3); border-bottom:1px solid var(--border); }
  .title { font-size:var(--text-sm); font-weight:600; color:var(--text); }
  .search { flex:1; height:26px; min-width:0; }
  .body { flex:1; overflow:auto; padding:var(--space-2) var(--space-3); }
  .section-label { font-size:var(--text-xs); font-weight:600; color:var(--text-faint); text-transform:uppercase; letter-spacing:0.5px; margin:var(--space-2) 0 var(--space-1); }
  .grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(170px,1fr)); gap:var(--space-2); }
  .card { display:flex; flex-direction:column; gap:2px; padding:var(--space-2) var(--space-3); background:var(--bg-subtle); border:1px solid var(--border-subtle); border-radius:var(--radius-sm); text-align:left; cursor:pointer; transition:border-color 80ms; }
  .card:hover { border-color:var(--text-faint); }
  .card.active { border-color:var(--accent); background:var(--accent-subtle); }
  .card.previewing { border-color:var(--warning); }
  .card-name { font-size:var(--text-sm); color:var(--text); font-weight:500; }
  .card-meta { font-size:var(--text-xs); color:var(--text-faint); display:flex; align-items:center; gap:var(--space-1); }
  .rm { background:transparent; border:0; color:var(--text-faint); padding:0; cursor:pointer; }
  .rm:hover { color:var(--danger); }
  .empty { padding:var(--space-4); color:var(--text-faint); text-align:center; font-size:var(--text-sm); }
</style>
