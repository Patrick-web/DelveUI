<script lang="ts">
  import { onMount } from "svelte";
  import { workspace, activeSession } from "./store";
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";
  import FileTreeNode from "./FileTreeNode.svelte";
  import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";

  type Entry = { name: string; path: string; isDir: boolean };

  let singleRoot: Entry[] = [];
  let singleRootPath = "";

  // Build list of all unique project directories from configs.
  $: projectDirs = (() => {
    const cfgs = $workspace?.configs ?? [];
    const seen = new Set<string>();
    const dirs: { label: string; path: string }[] = [];
    for (const cfg of cfgs) {
      const p = (cfg.cwd || cfg.program || "").replace(/\/+$/, "");
      if (p && !seen.has(p)) {
        seen.add(p);
        dirs.push({ label: cfg.label, path: p });
      }
    }
    return dirs;
  })();

  // For the single-project case we still expand contents inline so the user
  // doesn't have to click the project to see their files.
  $: singleProjectPath = projectDirs.length === 1
    ? projectDirs[0].path
    : (projectDirs.length === 0 ? deriveFallbackRoot($activeSession, $workspace) : "");
  $: if (singleProjectPath && singleProjectPath !== singleRootPath) {
    singleRootPath = singleProjectPath;
    loadSingleRoot();
  }

  function deriveFallbackRoot(session: any, ws: any): string {
    if (session?.cfg) {
      const cfg = session.cfg;
      if (cfg.cwd) return cfg.cwd.replace(/\/+$/, "");
      if (cfg.program) return cfg.program.replace(/\/+$/, "");
    }
    return ws?.root ?? "";
  }

  async function loadSingleRoot() {
    if (!singleRootPath) { singleRoot = []; return; }
    try { singleRoot = (await FileService.ListDir(singleRootPath)) as any[] ?? []; }
    catch { singleRoot = []; }
  }

  async function refresh() {
    if (singleRootPath) await loadSingleRoot();
    // Multi-project mode: each FileTreeNode reloads on its own toggle.
  }

  function shortPath(p: string) {
    return p.replace(/^\/Users\/[^/]+/, "~").split("/").slice(-2).join("/");
  }

  onMount(loadSingleRoot);

  export let hideHeader = false;
</script>

{#if !hideHeader}
<PanelHeader title="Files">
  <button class="btn icon" title="Refresh" on:click={refresh}>
    <Icon icon="solar:refresh-linear" size={13} />
  </button>
</PanelHeader>
{/if}

<div class="tree">
  {#if projectDirs.length === 0 && !singleRootPath}
    <div class="empty">Open a project to browse files.</div>
  {:else if projectDirs.length > 1}
    {#each projectDirs as d (d.path)}
      <FileTreeNode name={d.label} path={d.path} isDir={true} depth={0} />
    {/each}
  {:else}
    <div class="root-label">{shortPath(singleRootPath)}</div>
    {#each singleRoot as e (e.path)}
      <FileTreeNode name={e.name} path={e.path} isDir={e.isDir} depth={0} />
    {/each}
  {/if}
</div>

<style>
  .tree { flex:1; overflow:auto; }
  .empty { padding:var(--space-3); color:var(--text-faint); font-size:var(--text-sm); }
  .root-label { padding:var(--space-1) var(--space-2); font-size:var(--text-xs); color:var(--text-faint); font-family:var(--font-mono); border-bottom:1px solid var(--border-subtle); }
</style>
