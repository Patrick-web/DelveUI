<script lang="ts">
  import { onMount } from "svelte";
  import Icon from "./Icon.svelte";
  import { loadDebugFiles } from "./settings-store";
  import { refreshWorkspace } from "./store";
  import * as DetectService from "../../bindings/github.com/jp/DelveUI/internal/detect/service";

  export let open = false;

  type Source = {
    editor: string;
    projectPath: string;
    configPath: string;
    configCount: number;
    configs: any[];
    imported?: boolean;
    importing?: boolean;
    expanded?: boolean;
  };

  let sources: Source[] = [];
  let scanning = false;
  let scanned = false;

  $: if (open && !scanned) scan();

  const editorIcons: Record<string, string> = {
    "Zed": "devicon:zed",
    "VS Code": "devicon:vscode",
    "GoLand": "devicon:goland",
  };

  async function scan() {
    scanning = true;
    try {
      const raw = (await DetectService.Scan()) as any as Source[];
      sources = (raw ?? []).map((s) => ({
        ...s,
        imported: false,
        importing: false,
        expanded: false,
      }));
      // Check which are already imported
      for (const s of sources) {
        try {
          s.imported = (await DetectService.IsImported(s.configPath)) as boolean;
        } catch {}
      }
      sources = sources;
      scanned = true;
    } catch (e) {
      console.error("scan failed:", e);
    } finally {
      scanning = false;
    }
  }

  async function importSource(s: Source) {
    s.importing = true;
    sources = sources;
    try {
      if (s.editor === "GoLand") {
        await DetectService.ImportConfigs(s.projectPath, "goland", s.configs);
      } else {
        await DetectService.Import(s.configPath);
      }
      s.imported = true;
      await loadDebugFiles();
      await refreshWorkspace();
    } catch (e) {
      console.error("import failed:", e);
    } finally {
      s.importing = false;
      sources = sources;
    }
  }

  async function importAll() {
    for (const s of sources) {
      if (!s.imported) await importSource(s);
    }
  }

  // Append results from a hand-picked folder. Mirrors WelcomePage's scanFolder
  // — useful when a project lives outside the system-wide scan (deeply nested,
  // external drive, recently cloned).
  async function scanFolder() {
    try {
      const result = (await DetectService.PickAndScanFolder()) as any;
      if (!result?.projectPath) return;
      const found = (result.editorConfigs ?? []) as Source[];
      if (found.length === 0) {
        const { showInfo } = await import("./toast");
        showInfo(
          "No debug configs found",
          "You can still open this folder via the Open Workspace flow — DelveUI will auto-discover Go run targets.",
        );
        return;
      }
      const fresh: Source[] = [];
      for (const s of found) {
        if (sources.find((x) => x.configPath === s.configPath)) continue;
        let imported = false;
        try { imported = (await DetectService.IsImported(s.configPath)) as boolean; } catch {}
        fresh.push({ ...s, imported, importing: false, expanded: false });
      }
      if (fresh.length > 0) {
        sources = [...sources, ...fresh];
        scanned = true;
      }
    } catch (e) {
      const { showError } = await import("./toast");
      showError("Folder scan failed", String((e as any)?.message ?? e));
    }
  }

  function shortPath(p: string) {
    const home = p.replace(/^\/Users\/[^/]+/, "~");
    return home;
  }

  function close() {
    open = false;
  }

  // Workspaces: collapse sources into one entry per detected project path.
  // Same pattern as WelcomePage — workspace is the unit, editors are detail.
  type Workspace = {
    path: string;
    sources: Source[];
    totalConfigs: number;
    expanded: boolean;
  };
  let expandedPaths = new Set<string>();
  function toggleExpanded(path: string) {
    if (expandedPaths.has(path)) expandedPaths.delete(path);
    else expandedPaths.add(path);
    expandedPaths = expandedPaths;
  }

  $: workspaces = (() => {
    const map = new Map<string, Source[]>();
    for (const s of sources) {
      const list = map.get(s.projectPath) ?? [];
      list.push(s);
      map.set(s.projectPath, list);
    }
    const out: Workspace[] = [];
    for (const [path, srcs] of map.entries()) {
      out.push({
        path,
        sources: srcs,
        totalConfigs: srcs.reduce((n, s) => n + (s.configCount ?? 0), 0),
        expanded: expandedPaths.has(path),
      });
    }
    return out;
  })();

  function workspaceImported(w: Workspace): boolean {
    return w.sources.length > 0 && w.sources.every(s => s.imported);
  }
  function workspaceImporting(w: Workspace): boolean {
    return w.sources.some(s => s.importing);
  }
  async function importWorkspace(w: Workspace) {
    for (const s of w.sources) {
      if (!s.imported) await importSource(s);
    }
  }
  function workspaceName(path: string): string {
    const parts = path.split("/").filter(Boolean);
    return parts[parts.length - 1] ?? path;
  }
</script>

{#if open}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div class="backdrop" role="presentation" on:click={close}></div>
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div class="wizard" role="dialog" aria-modal="true" on:keydown={(e) => e.key === "Escape" && close()}>
    <header>
      <Icon icon="solar:magnifer-bold" size={16} color="var(--accent)" />
      <h2>Import Debug Configurations</h2>
      <div class="header-actions">
        <button class="btn outlined" on:click={scanFolder} title="Scan a specific folder you choose">
          <Icon icon="solar:folder-with-files-bold" size={12} />
          Scan Folder…
        </button>
        <button class="btn outlined" on:click={scan} disabled={scanning}>
          <Icon icon="solar:refresh-linear" size={12} />
          {scanning ? "Scanning…" : "Rescan"}
        </button>
        <button class="btn icon" on:click={close} title="Close">
          <Icon icon="solar:close-circle-linear" size={14} />
        </button>
      </div>
    </header>

    <div class="body">
      {#if scanning}
        <div class="status">
          <div class="spinner"></div>
          <span>Scanning your system for Go projects with debug configs…</span>
          <span class="sub">This may take a moment on first run</span>
        </div>
      {:else if sources.length === 0}
        <div class="status">
          <Icon icon="solar:inbox-bold" size={20} color="var(--text-faint)" />
          <span>No debug configurations found.</span>
          <span class="sub">Make sure your Go projects have <code>.vscode/launch.json</code>, <code>.zed/debug.json</code>, or <code>.idea/runConfigurations/</code></span>
        </div>
      {:else}
        <div class="results">
          {#each workspaces as w (w.path)}
            {@const imported = workspaceImported(w)}
            {@const importing = workspaceImporting(w)}
            <div class="workspace" class:expanded={w.expanded}>
              <!-- svelte-ignore a11y-click-events-have-key-events -->
              <!-- svelte-ignore a11y-no-static-element-interactions -->
              <div class="workspace-row" on:click={() => toggleExpanded(w.path)} role="button" tabindex="0">
                <span class="chev"><Icon icon="solar:alt-arrow-right-linear" size={10} /></span>
                <Icon icon="solar:folder-bold" size={14} color="var(--text-muted)" />
                <div class="workspace-info">
                  <span class="workspace-name">{workspaceName(w.path)}</span>
                  <span class="workspace-path">{shortPath(w.path)}</span>
                </div>
                <div class="editor-stack">
                  {#each w.sources as s}
                    <Icon icon={editorIcons[s.editor] ?? "solar:code-bold"} size={14} />
                  {/each}
                </div>
                <span class="config-count">{w.totalConfigs} config{w.totalConfigs !== 1 ? "s" : ""}</span>
                {#if imported}
                  <span class="imported-badge">
                    <Icon icon="solar:check-circle-bold" size={14} color="var(--success)" />
                    Imported
                  </span>
                {:else}
                  <button class="btn primary" on:click|stopPropagation={() => importWorkspace(w)} disabled={importing}>
                    {importing ? "Importing…" : "Import"}
                  </button>
                {/if}
              </div>
              {#if w.expanded}
                <div class="workspace-detail">
                  {#each w.sources as s}
                    <div class="editor-row">
                      <Icon icon={editorIcons[s.editor] ?? "solar:code-bold"} size={14} />
                      <span class="editor-name">{s.editor}</span>
                      <span class="config-count">{s.configCount} config{s.configCount !== 1 ? "s" : ""}</span>
                      {#if s.imported}
                        <span class="imported-badge sm">
                          <Icon icon="solar:check-circle-bold" size={12} color="var(--success)" />
                          Imported
                        </span>
                      {:else}
                        <button class="btn sm" on:click|stopPropagation={() => importSource(s)} disabled={s.importing}>
                          {s.importing ? "…" : "Import"}
                        </button>
                      {/if}
                    </div>
                    <div class="config-list">
                      {#each s.configs as cfg}
                        <div class="config-item">
                          <Icon icon="solar:play-bold" size={10} color="var(--success)" />
                          <span class="cfg-name">{cfg.label}</span>
                          <span class="cfg-mode">{cfg.mode ?? "debug"}</span>
                        </div>
                      {/each}
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <footer>
      <span class="count">{sources.length} source{sources.length !== 1 ? "s" : ""} found</span>
      <button class="btn primary" on:click={importAll} disabled={scanning || sources.every(s => s.imported)}>
        Import All
      </button>
    </footer>
  </div>
{/if}

<style>
  .backdrop { position:fixed; inset:0; background:rgba(0,0,0,0.45); z-index:800; }
  .wizard {
    position:fixed; top:50%; left:50%; transform:translate(-50%,-50%);
    width:640px; max-height:80vh;
    background:var(--bg); border:1px solid var(--border); border-radius:var(--radius-md);
    display:flex; flex-direction:column; overflow:hidden;
    box-shadow:0 24px 64px rgba(0,0,0,0.5); z-index:801;
  }
  header {
    display:flex; align-items:center; gap:var(--space-2);
    padding:var(--space-3) var(--space-4); border-bottom:1px solid var(--border);
  }
  header h2 { font-size:var(--text-md); font-weight:600; color:var(--text); margin:0; flex:1; }
  .header-actions { display:flex; gap:var(--space-1); }

  .body { flex:1; overflow:auto; padding:var(--space-3); }

  .status { display:flex; flex-direction:column; align-items:center; gap:var(--space-2); padding:var(--space-8) var(--space-4); color:var(--text-muted); font-size:var(--text-sm); text-align:center; }
  .status .sub { font-size:var(--text-xs); color:var(--text-faint); }
  .status code { background:var(--bg-subtle); padding:1px 4px; border-radius:3px; font-family:var(--font-mono); font-size:var(--text-xs); }

  .spinner {
    width: 24px; height: 24px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  .results { display:flex; flex-direction:column; gap:6px; }

  .workspace { display:flex; flex-direction:column; }
  .workspace-row {
    display:flex; align-items:center; gap:var(--space-2);
    padding:var(--space-2) var(--space-3);
    background:var(--bg-subtle); border:1px solid var(--border-subtle);
    border-radius:var(--radius-sm); cursor:pointer;
    transition:background 80ms ease;
  }
  .workspace-row:hover { background:rgba(255,255,255,0.04); }
  .chev { display:inline-flex; color:var(--text-faint); transition:transform 120ms ease; }
  .workspace.expanded .chev { transform:rotate(90deg); }
  .workspace-info { flex:1; min-width:0; display:flex; align-items:baseline; gap:8px; overflow:hidden; }
  .workspace-name { font-size:var(--text-sm); font-weight:600; color:var(--text); flex-shrink:0; }
  .workspace-path { font-family:var(--font-mono); font-size:11px; color:var(--text-faint); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .editor-stack { display:inline-flex; align-items:center; gap:4px; flex-shrink:0; }
  .editor-name { font-size:var(--text-sm); font-weight:500; color:var(--text); }
  .config-count { font-size:var(--text-xs); color:var(--text-faint); flex-shrink:0; }
  .imported-badge { display:flex; align-items:center; gap:var(--space-1); font-size:var(--text-xs); color:var(--success); flex-shrink:0; }
  .imported-badge.sm { font-size:11px; }
  .sm { padding:3px 10px; font-size:var(--text-xs); }

  .workspace-detail {
    margin:4px 0 0 24px;
    padding:var(--space-2);
    background:var(--bg); border:1px solid var(--border-subtle); border-radius:var(--radius-sm);
    display:flex; flex-direction:column; gap:6px;
  }
  .editor-row { display:flex; align-items:center; gap:var(--space-2); padding:2px 4px; }

  .config-list { padding:0 0 0 var(--space-5); display:flex; flex-direction:column; }
  .config-item { display:flex; align-items:center; gap:var(--space-2); padding:1px 0; font-size:var(--text-xs); }
  .cfg-name { color:var(--text); font-family:var(--font-mono); }
  .cfg-mode { color:var(--text-faint); }

  footer {
    display:flex; align-items:center; justify-content:space-between;
    padding:var(--space-2) var(--space-4); border-top:1px solid var(--border);
  }
  .count { font-size:var(--text-xs); color:var(--text-faint); }
</style>
