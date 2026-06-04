<script lang="ts">
  import { onMount } from "svelte";
  import Icon from "./Icon.svelte";
  import { Browser } from "@wailsio/runtime";
  import { marked } from "marked";

  export let open = false;
  export let currentVersion = "";
  export let latestVersion = "";
  export let releaseNotes = "";
  export let releaseUrl = "";

  let html = "";

  onMount(() => {
    if (releaseNotes) {
      html = marked.parse(releaseNotes, { async: false }) as string;
    }
  });

  function close() { open = false; }

  function openInBrowser() {
    if (releaseUrl) Browser.OpenURL(releaseUrl);
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Escape") { e.preventDefault(); close(); }
  }

  function onBodyClick(e: MouseEvent) {
    const t = e.target as HTMLElement;
    const a = t.closest("a") as HTMLAnchorElement | null;
    if (a && a.href && a.hostname) {
      e.preventDefault();
      Browser.OpenURL(a.href);
    }
  }
</script>

{#if open}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="backdrop" role="presentation" on:click={close}></div>
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div class="modal" role="dialog" aria-modal="true" aria-label="Release notes" on:keydown={onKey}>
    <div class="head">
      <Icon icon="solar:document-text-linear" size={16} color="var(--accent)" />
      <div class="head-text">
        <span class="head-title">Release Notes — v{latestVersion}</span>
        <span class="head-sub">You're on v{currentVersion}</span>
      </div>
      <button class="btn icon" title="Open in browser" on:click={openInBrowser}>
        <Icon icon="solar:link-square-linear" size={14} />
      </button>
      <button class="btn icon" title="Close (Esc)" on:click={close}>
        <Icon icon="solar:close-circle-linear" size={16} />
      </button>
    </div>
    <div class="body">
      {#if html}
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="rendered" on:click={onBodyClick}>{@html html}</div>
      {:else}
        <div class="empty">
          <Icon icon="solar:document-text-linear" size={24} color="var(--text-faint)" />
          <p>No release notes available.</p>
        </div>
      {/if}
    </div>
    <div class="foot">
      <button class="btn primary sm" on:click={openInBrowser}>
        <Icon icon="solar:link-square-linear" size={11} /> Open on GitHub
      </button>
      <button class="btn outlined sm" on:click={close}>Close</button>
    </div>
  </div>
{/if}

<style>
  .backdrop { position:fixed; inset:0; background:rgba(0,0,0,0.45); z-index:900; }
  .modal {
    position:fixed; inset:60px; max-width:700px; margin:0 auto;
    background:var(--bg); border:1px solid var(--border);
    border-radius:var(--radius-md); display:flex; flex-direction:column;
    overflow:hidden; box-shadow:0 24px 64px rgba(0,0,0,0.5); z-index:901;
  }
  .head {
    display:flex; align-items:center; gap:var(--space-2);
    padding:var(--space-3) var(--space-4);
    border-bottom:1px solid var(--border);
    background:var(--bg-subtle); flex-shrink:0;
  }
  .head-text { flex:1; display:flex; flex-direction:column; gap:1px; }
  .head-title { font-size:var(--text-sm); font-weight:700; color:var(--text); }
  .head-sub { font-size:var(--text-xs); color:var(--text-faint); }
  .body {
    flex:1; overflow:auto; padding:var(--space-4) var(--space-5);
    line-height:1.6;
  }
  .rendered :global(h1) { font-size:16px; font-weight:700; color:var(--text); margin:16px 0 8px; border-bottom:1px solid var(--border-subtle); padding-bottom:4px; }
  .rendered :global(h2) { font-size:14px; font-weight:700; color:var(--text); margin:12px 0 6px; }
  .rendered :global(h3) { font-size:13px; font-weight:600; color:var(--text); margin:10px 0 4px; }
  .rendered :global(p) { font-size:var(--text-sm); color:var(--text-muted); margin:6px 0; }
  .rendered :global(ul), .rendered :global(ol) { padding-left:20px; margin:6px 0; }
  .rendered :global(li) { font-size:var(--text-sm); color:var(--text-muted); margin:3px 0; }
  .rendered :global(li > p) { margin:2px 0; }
  .rendered :global(a) { color:var(--accent); text-decoration:none; cursor:pointer; }
  .rendered :global(a:hover) { text-decoration:underline; }
  .rendered :global(code) {
    font-family:var(--font-mono); font-size:var(--text-xs);
    background:var(--bg-subtle); padding:1px 5px; border-radius:3px;
    color:var(--text);
  }
  .rendered :global(pre) {
    background:var(--bg-subtle); border:1px solid var(--border-subtle);
    border-radius:var(--radius-sm); padding:var(--space-3); margin:8px 0;
    overflow-x:auto;
  }
  .rendered :global(pre code) {
    background:transparent; padding:0; border-radius:0;
    font-size:var(--text-xs); line-height:1.5;
  }
  .rendered :global(blockquote) {
    border-left:3px solid var(--accent); margin:8px 0; padding:4px 12px;
    color:var(--text-faint); font-style:italic;
  }
  .rendered :global(hr) { border:none; border-top:1px solid var(--border-subtle); margin:12px 0; }
  .rendered :global(img) { max-width:100%; border-radius:var(--radius-sm); margin:8px 0; }
  .rendered :global(table) { width:100%; border-collapse:collapse; margin:8px 0; font-size:var(--text-sm); }
  .rendered :global(th), .rendered :global(td) { border:1px solid var(--border-subtle); padding:6px 10px; text-align:left; }
  .rendered :global(th) { background:var(--bg-subtle); font-weight:600; color:var(--text); }
  .rendered :global(input[type="checkbox"]) { accent-color:var(--accent); }
  .empty {
    display:flex; flex-direction:column; align-items:center; gap:var(--space-2);
    padding:48px 0; color:var(--text-faint); text-align:center;
  }
  .foot {
    display:flex; justify-content:flex-end; gap:var(--space-2);
    padding:var(--space-2) var(--space-4);
    border-top:1px solid var(--border);
    background:var(--bg-subtle); flex-shrink:0;
  }
  .btn { display:inline-flex; align-items:center; gap:4px; }
</style>
