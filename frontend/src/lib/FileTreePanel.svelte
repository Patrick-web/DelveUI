<script lang="ts">
  import { onMount } from "svelte";
  import { workspace } from "./store";
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";
  import FileTreeNode from "./FileTreeNode.svelte";
  import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";

  type Entry = { name: string; path: string; isDir: boolean };

  let root: Entry[] = [];
  let rootPath = "";

  $: newRoot = $workspace?.root ?? "";
  $: if (newRoot && newRoot !== rootPath) { rootPath = newRoot; loadRoot(); }

  async function loadRoot() {
    if (!rootPath) return;
    try { root = (await FileService.ListDir(rootPath)) as any[] ?? []; } catch { root = []; }
  }

  function shortRoot(p: string) { return p.replace(/^\/Users\/[^/]+/, "~"); }

  onMount(loadRoot);
</script>

<PanelHeader title="Files">
  <button class="btn icon" title="Refresh" on:click={loadRoot}>
    <Icon icon="solar:refresh-linear" size={13} />
  </button>
</PanelHeader>

<div class="tree">
  {#if !rootPath}
    <div class="empty">Open a project to browse files.</div>
  {:else}
    <div class="root-label">{shortRoot(rootPath)}</div>
    {#each root as e (e.path)}
      <FileTreeNode name={e.name} path={e.path} isDir={e.isDir} depth={0} />
    {/each}
  {/if}
</div>

<style>
  .tree { flex:1; overflow:auto; }
  .empty { padding:var(--space-3); color:var(--text-faint); font-size:var(--text-sm); }
  .root-label { padding:var(--space-1) var(--space-2); font-size:var(--text-xs); color:var(--text-faint); font-family:var(--font-mono); border-bottom:1px solid var(--border-subtle); }
</style>
