<script lang="ts">
  import { manualSourcePath } from "./store";
  import { setActivePanel } from "./panels/layout";
  import Icon from "./Icon.svelte";
  import Self from "./FileTreeNode.svelte";
  import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";

  export let name: string;
  export let path: string;
  export let isDir: boolean;
  export let depth: number = 0;

  let expanded = false;
  let children: any[] | null = null;
  let loading = false;

  async function toggle() {
    if (!isDir) {
      manualSourcePath.set(path);
      setActivePanel("right", "source");
      return;
    }
    expanded = !expanded;
    if (expanded && !children) {
      loading = true;
      try {
        children = (await FileService.ListDir(path)) as any[] ?? [];
      } catch { children = []; }
      loading = false;
    }
  }

  function fileIcon(n: string): string {
    if (n.endsWith(".go")) return "solar:code-bold";
    if (n.endsWith(".json")) return "solar:document-bold";
    if (n.endsWith(".mod") || n.endsWith(".sum")) return "solar:box-bold";
    return "solar:file-bold";
  }
</script>

<button class="node" style:padding-left="{depth * 14 + 8}px" on:click={toggle}>
  {#if isDir}
    <Icon icon={expanded ? "solar:folder-open-bold" : "solar:folder-bold"} size={13} color="var(--text-muted)" />
  {:else}
    <Icon icon={fileIcon(name)} size={13} color={name.endsWith(".go") ? "var(--info)" : "var(--text-faint)"} />
  {/if}
  <span class="name" class:go={name.endsWith(".go")}>{name}</span>
  {#if loading}<span class="ld">…</span>{/if}
</button>

{#if expanded && children}
  {#each children as c (c.path)}
    <Self name={c.name} path={c.path} isDir={c.isDir} depth={depth + 1} />
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
  .name { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .name.go { color:var(--info); }
  .ld { color:var(--text-faint); font-size:var(--text-xs); }
</style>
