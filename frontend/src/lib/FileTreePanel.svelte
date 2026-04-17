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

  // Derive the best project root:
  // 1. Active session's cwd or program directory (the actual code being debugged)
  // 2. First config's cwd or program directory
  // 3. Fall back to workspace.root (debug.json location)
  $: projectRoot = deriveRoot($activeSession, $workspace);
  $: if (projectRoot && projectRoot !== rootPath) { rootPath = projectRoot; loadRoot(); }

  function deriveRoot(session: any, ws: any): string {
    // From active session's config
    if (session?.cfg) {
      const cfg = session.cfg;
      if (cfg.cwd) return cfg.cwd.replace(/\/+$/, "");
      if (cfg.program) return cfg.program.replace(/\/+$/, "");
    }
    // From first available config
    const cfgs = ws?.configs ?? [];
    if (cfgs.length > 0) {
      const cfg = cfgs[0];
      if (cfg.cwd) return cfg.cwd.replace(/\/+$/, "");
      if (cfg.program) return cfg.program.replace(/\/+$/, "");
    }
    // Fall back to workspace root
    return ws?.root ?? "";
  }

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
