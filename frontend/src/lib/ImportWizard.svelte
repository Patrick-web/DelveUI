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

  function shortPath(p: string) {
    const home = p.replace(/^\/Users\/[^/]+/, "~");
    return home;
  }

  function close() {
    open = false;
  }

  // Group by project
  $: grouped = (() => {
    const map = new Map<string, Source[]>();
    for (const s of sources) {
      const list = map.get(s.projectPath) ?? [];
      list.push(s);
      map.set(s.projectPath, list);
    }
    return [...map.entries()];
  })();
</script>

{#if open}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="backdrop" role="presentation" on:click={close}></div>
  <div class="wizard" role="dialog" aria-modal="true" on:keydown={(e) => e.key === "Escape" && close()}>
    <header>
      <Icon icon="solar:magnifer-bold" size={16} color="var(--accent)" />
      <h2>Import Debug Configurations</h2>
      <div class="header-actions">
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
          {#each grouped as [projectPath, projectSources]}
            <div class="project">
              <div class="project-header">
                <Icon icon="solar:folder-bold" size={14} color="var(--text-muted)" />
                <span class="project-path">{shortPath(projectPath)}</span>
              </div>
              {#each projectSources as s}
                <div class="source">
                  <Icon icon={editorIcons[s.editor] ?? "solar:code-bold"} size={16} />
                  <div class="source-info">
                    <span class="editor-name">{s.editor}</span>
                    <span class="config-count">{s.configCount} config{s.configCount !== 1 ? "s" : ""}</span>
                  </div>
                  {#if s.imported}
                    <span class="imported-badge">
                      <Icon icon="solar:check-circle-bold" size={14} color="var(--success)" />
                      Imported
                    </span>
                  {:else}
                    <button class="btn primary" on:click={() => importSource(s)} disabled={s.importing}>
                      {s.importing ? "Importing…" : "Import"}
                    </button>
                  {/if}
                  <button class="btn icon" on:click={() => { s.expanded = !s.expanded; sources = sources; }} title="Show configs">
                    <Icon icon={s.expanded ? "solar:alt-arrow-up-linear" : "solar:alt-arrow-down-linear"} size={12} />
                  </button>
                </div>
                {#if s.expanded}
                  <div class="config-list">
                    {#each s.configs as cfg}
                      <div class="config-item">
                        <Icon icon="solar:play-bold" size={10} color="var(--success)" />
                        <span class="cfg-name">{cfg.label}</span>
                        <span class="cfg-mode">{cfg.mode ?? "debug"}</span>
                      </div>
                    {/each}
                  </div>
                {/if}
              {/each}
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

  .project { margin-bottom:var(--space-3); }
  .project-header { display:flex; align-items:center; gap:var(--space-2); padding:var(--space-1) 0; margin-bottom:var(--space-1); }
  .project-path { font-family:var(--font-mono); font-size:var(--text-xs); color:var(--text-muted); }

  .source {
    display:flex; align-items:center; gap:var(--space-2);
    padding:var(--space-2) var(--space-3);
    background:var(--bg-subtle); border:1px solid var(--border-subtle);
    border-radius:var(--radius-sm); margin-bottom:2px;
  }
  .source-info { flex:1; display:flex; align-items:center; gap:var(--space-2); }
  .editor-name { font-size:var(--text-sm); font-weight:500; color:var(--text); }
  .config-count { font-size:var(--text-xs); color:var(--text-faint); }
  .imported-badge { display:flex; align-items:center; gap:var(--space-1); font-size:var(--text-xs); color:var(--success); }

  .config-list { padding:var(--space-1) 0 var(--space-1) var(--space-6); }
  .config-item { display:flex; align-items:center; gap:var(--space-2); padding:2px 0; font-size:var(--text-xs); }
  .cfg-name { color:var(--text); font-family:var(--font-mono); }
  .cfg-mode { color:var(--text-faint); }

  footer {
    display:flex; align-items:center; justify-content:space-between;
    padding:var(--space-2) var(--space-4); border-top:1px solid var(--border);
  }
  .count { font-size:var(--text-xs); color:var(--text-faint); }
</style>
