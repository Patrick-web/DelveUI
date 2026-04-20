<script lang="ts">
  import { onMount } from "svelte";
  import { workspace, activeSession, sessions } from "./store";
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";
  import FileTreeNode from "./FileTreeNode.svelte";
  import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";

  type Entry = { name: string; path: string; isDir: boolean };

  let root: Entry[] = [];
  let rootPath = "";
  let manualRoot = ""; // user-selected override

  // Build list of all unique project directories from configs
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

  // Active root: manual selection > active session > first config > workspace root
  $: projectRoot = manualRoot || deriveRoot($activeSession, $workspace);
  $: if (projectRoot && projectRoot !== rootPath) { rootPath = projectRoot; loadRoot(); }

  function deriveRoot(session: any, ws: any): string {
    if (session?.cfg) {
      const cfg = session.cfg;
      if (cfg.cwd) return cfg.cwd.replace(/\/+$/, "");
      if (cfg.program) return cfg.program.replace(/\/+$/, "");
    }
    const cfgs = ws?.configs ?? [];
    if (cfgs.length > 0) {
      const cfg = cfgs[0];
      if (cfg.cwd) return cfg.cwd.replace(/\/+$/, "");
      if (cfg.program) return cfg.program.replace(/\/+$/, "");
    }
    return ws?.root ?? "";
  }

  function switchRoot(path: string) {
    manualRoot = path;
  }

  async function loadRoot() {
    if (!rootPath) return;
    try { root = (await FileService.ListDir(rootPath)) as any[] ?? []; } catch { root = []; }
  }

  function shortPath(p: string) {
    return p.replace(/^\/Users\/[^/]+/, "~").split("/").slice(-2).join("/");
  }

  onMount(loadRoot);

  export let hideHeader = false;
</script>

{#if !hideHeader}
<PanelHeader title="Files">
  <button class="btn icon" title="Refresh" on:click={loadRoot}>
    <Icon icon="solar:refresh-linear" size={13} />
  </button>
</PanelHeader>
{/if}

{#if projectDirs.length > 1}
  <div class="switcher">
    <select class="dir-select" value={rootPath} on:change={(e) => switchRoot(e.currentTarget.value)}>
      {#each projectDirs as d}
        <option value={d.path}>{d.label} — {shortPath(d.path)}</option>
      {/each}
    </select>
  </div>
{/if}

<div class="tree">
  {#if !rootPath}
    <div class="empty">Open a project to browse files.</div>
  {:else}
    {#if projectDirs.length <= 1}
      <div class="root-label">{shortPath(rootPath)}</div>
    {/if}
    {#each root as e (e.path)}
      <FileTreeNode name={e.name} path={e.path} isDir={e.isDir} depth={0} />
    {/each}
  {/if}
</div>

<style>
  .tree { flex:1; overflow:auto; }
  .empty { padding:var(--space-3); color:var(--text-faint); font-size:var(--text-sm); }
  .root-label { padding:var(--space-1) var(--space-2); font-size:var(--text-xs); color:var(--text-faint); font-family:var(--font-mono); border-bottom:1px solid var(--border-subtle); }
  .switcher { padding:var(--space-1) var(--space-2); border-bottom:1px solid var(--border-subtle); }
  .dir-select {
    width:100%; background:var(--bg-subtle); border:1px solid var(--border-subtle);
    color:var(--text); font-size:var(--text-xs); font-family:var(--font-mono);
    padding:3px 4px; border-radius:var(--radius-sm); cursor:pointer; outline:none;
  }
  .dir-select:focus { border-color:var(--accent); }
</style>
