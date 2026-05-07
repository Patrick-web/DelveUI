<script lang="ts">
  import { manualSourcePath } from "./store";
  import { setActivePanel } from "./panels/layout";
  import Icon from "./Icon.svelte";
  import Self from "./FileTreeNode.svelte";
  import { getFileIcon, getFolderIcon } from "./file-icons";
  import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";

  export let name: string;
  export let path: string;
  export let isDir: boolean;
  export let depth: number = 0;
  export let focusPath: string = "";

  let expanded = false;
  let children: any[] | null = null;
  let loading = false;

  $: isActive = !isDir && path === focusPath;

  async function loadChildren() {
    loading = true;
    try {
      children = (await FileService.ListDir(path)) as any[] ?? [];
    } catch { children = []; }
    loading = false;
  }

  $: if (focusPath && isDir && focusPath !== path) {
    const prefix = path.endsWith("/") ? path : path + "/";
    if (focusPath.startsWith(prefix)) {
      if (!expanded) expanded = true;
      if (!children) loadChildren();
    }
  }

  async function toggle() {
    if (!isDir) {
      manualSourcePath.set(path);
      setActivePanel("right", "source");
      return;
    }
    expanded = !expanded;
    if (expanded && !children) {
      await loadChildren();
    }
  }
</script>

<button class="node" class:active={isActive} style:padding-left="{depth * 14 + 8}px" on:click={toggle}>
  {#if isDir}
    <Icon icon={getFolderIcon(name, expanded)} size={13} />
  {:else}
    <Icon icon={getFileIcon(name)} size={13} />
  {/if}
  <span class="name">{name}</span>
  {#if loading}<span class="ld">…</span>{/if}
</button>

{#if expanded && children}
  {#each children as c (c.path)}
    <Self name={c.name} path={c.path} isDir={c.isDir} depth={depth + 1} focusPath={focusPath} />
  {/each}
{/if}

<style>
  .node {
    display:flex; align-items:center; gap:4px; width:100%;
    background:transparent; border:0; color:var(--text); cursor:pointer;
    padding:2px 8px; font-family:var(--font-mono); font-size:var(--text-sm);
    text-align:left;
  }
  .node:hover { background:var(--bg-subtle); }
  .node.active { background:rgba(59,130,246,0.15); }
  .name { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .ld { color:var(--text-faint); font-size:var(--text-xs); }
</style>
