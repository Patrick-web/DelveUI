<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Events } from "@wailsio/runtime";
  import Icon from "./Icon.svelte";
  import { loadDebugFiles } from "./settings-store";
  import { refreshWorkspace, startSession } from "./store";
  import * as DetectService from "../../bindings/github.com/jp/DelveUI/internal/detect/service";

  export let visible = false;
  export let onDone: () => void = () => {};

  type Source = {
    editor: string;
    projectPath: string;
    configPath: string;
    configCount: number;
    configs: any[];
    imported: boolean;
    importing: boolean;
  };

  type RunTarget = {
    label: string;
    kind: string;
    package: string;
    dir: string;
  };

  let sources: Source[] = [];
  let scanning = false;
  let scanStatus = "";
  let scanDone = false;

  type Tab = "debug" | "run";
  let tab: Tab = "debug";

  // Run targets from folder scans
  let runTargets: RunTarget[] = [];

  const editorIcons: Record<string, string> = {
    "Zed": "devicon:zed",
    "VS Code": "devicon:vscode",
    "GoLand": "devicon:goland",
  };

  onMount(() => {
    if (visible) scan();
  });

  $: if (visible && !scanDone && !scanning) scan();

  // Listen for streaming scan events
  let unsubs: (() => void)[] = [];
  onMount(() => {
    unsubs.push(Events.On("scan:progress", (ev: any) => {
      const d = ev.data;
      scanStatus = d?.dir ?? "";
    }));
    unsubs.push(Events.On("scan:result", (ev: any) => {
      const s = ev.data as Source;
      if (s && !sources.find(x => x.configPath === s.configPath)) {
        sources = [...sources, { ...s, imported: false, importing: false }];
      }
    }));
    unsubs.push(Events.On("scan:done", () => {
      scanning = false;
      scanDone = true;
    }));
  });
  onDestroy(() => unsubs.forEach(u => u()));

  async function scan() {
    scanning = true;
    scanDone = false;
    sources = [];
    scanStatus = "Starting scan…";
    try {
      const raw = (await DetectService.Scan()) as any as Source[];
      // Results already streamed via events, but merge any we missed
      for (const s of raw ?? []) {
        if (!sources.find(x => x.configPath === s.configPath)) {
          sources = [...sources, { ...s, imported: false, importing: false }];
        }
      }
      // Check imported status
      for (const s of sources) {
        try { s.imported = (await DetectService.IsImported(s.configPath)) as boolean; } catch {}
      }
      sources = sources;
    } catch (e) {
      console.error("scan failed:", e);
    } finally {
      scanning = false;
      scanDone = true;
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
    } catch (e) { console.error(e); }
    finally { s.importing = false; sources = sources; }
  }

  async function importAll() {
    for (const s of sources) { if (!s.imported) await importSource(s); }
  }

  async function importFolder() {
    // This would ideally use a native folder picker but for now use the service
    // The user can also use the Import Wizard for folder scanning
  }

  function skip() { onDone(); }
  function finish() { onDone(); }

  function shortPath(p: string) { return p.replace(/^\/Users\/[^/]+/, "~"); }

  $: grouped = (() => {
    const map = new Map<string, Source[]>();
    for (const s of sources) {
      const list = map.get(s.projectPath) ?? [];
      list.push(s);
      map.set(s.projectPath, list);
    }
    return [...map.entries()];
  })();

  $: importedCount = sources.filter(s => s.imported).length;
</script>

{#if visible}
  <div class="welcome">
    <div class="container">
      <div class="header">
        <Icon icon="solar:code-2-bold-duotone" size={36} color="var(--accent)" />
        <div>
          <h1>Welcome to DelveUI</h1>
          <p class="subtitle">Delve debugger GUI for Go</p>
        </div>
      </div>

      <!-- Tabs: Run | Debug (like Zed) -->
      <div class="tabs">
        <button class:active={tab === "debug"} on:click={() => (tab = "debug")}>
          <Icon icon="solar:bug-bold" size={13} /> Debug
          {#if sources.length > 0}
            <span class="badge">{sources.length}</span>
          {/if}
        </button>
        <button class:active={tab === "run"} on:click={() => (tab = "run")}>
          <Icon icon="solar:play-bold" size={13} /> Run
        </button>
      </div>

      {#if tab === "debug"}
        <!-- Scan status -->
        {#if scanning}
          <div class="scan-bar">
            <div class="spinner"></div>
            <span class="scan-text">{scanStatus}</span>
            <span class="scan-count">{sources.length} found</span>
          </div>
        {:else if scanDone && sources.length === 0}
          <div class="empty">
            <Icon icon="solar:inbox-bold" size={20} color="var(--text-faint)" />
            <span>No debug configurations found on your system.</span>
            <span class="sub">Create a <code>.vscode/launch.json</code> or <code>.zed/debug.json</code> in your Go project.</span>
          </div>
        {/if}

        <!-- Results (shown even while scanning) -->
        {#if sources.length > 0}
          <div class="source-list">
            {#each grouped as [projectPath, projectSources]}
              <div class="project">
                <div class="project-header">
                  <Icon icon="solar:folder-bold" size={13} color="var(--text-muted)" />
                  <span class="project-path">{shortPath(projectPath)}</span>
                </div>
                {#each projectSources as s}
                  <div class="source-row">
                    <Icon icon={editorIcons[s.editor] ?? "solar:code-bold"} size={16} />
                    <span class="editor">{s.editor}</span>
                    <span class="count">{s.configCount} config{s.configCount !== 1 ? "s" : ""}</span>
                    {#if s.imported}
                      <span class="imported"><Icon icon="solar:check-circle-bold" size={13} color="var(--success)" /> Done</span>
                    {:else}
                      <button class="btn primary sm" on:click={() => importSource(s)} disabled={s.importing}>
                        {s.importing ? "…" : "Import"}
                      </button>
                    {/if}
                  </div>
                {/each}
              </div>
            {/each}
          </div>
        {/if}

      {:else if tab === "run"}
        <div class="empty">
          <Icon icon="solar:play-circle-bold" size={20} color="var(--text-faint)" />
          <span>Auto-detected Go targets will appear here.</span>
          <span class="sub">Import a project first, or use <strong>Import Folder</strong> to scan any directory for <code>func main()</code> and test files.</span>
        </div>
      {/if}

      <!-- Actions -->
      <div class="actions">
        {#if sources.length > 0 && importedCount > 0}
          <button class="btn primary" on:click={finish}>
            <Icon icon="solar:arrow-right-bold" size={13} /> Get Started
          </button>
        {:else if sources.length > 0}
          <button class="btn primary" on:click={importAll}>
            <Icon icon="solar:download-minimalistic-bold" size={13} /> Import All
          </button>
        {/if}
        <button class="btn" on:click={skip}>Skip</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .welcome { position:fixed; inset:0; z-index:700; background:var(--bg); display:flex; align-items:center; justify-content:center; }
  .container { width:100%; max-width:580px; padding:var(--space-6); display:flex; flex-direction:column; gap:var(--space-4); }

  .header { display:flex; align-items:center; gap:var(--space-3); }
  h1 { font-size:20px; font-weight:700; color:var(--text); margin:0; }
  .subtitle { color:var(--text-muted); font-size:var(--text-sm); margin:0; }

  .tabs { display:flex; gap:0; border-bottom:1px solid var(--border); }
  .tabs button { display:flex; align-items:center; gap:var(--space-1); background:transparent; border:0; border-bottom:2px solid transparent; color:var(--text-muted); padding:var(--space-2) var(--space-3); font-size:var(--text-sm); cursor:pointer; }
  .tabs button:hover { color:var(--text); }
  .tabs button.active { color:var(--text); border-bottom-color:var(--accent); }
  .badge { background:var(--accent); color:#fff; font-size:9px; padding:1px 5px; border-radius:8px; margin-left:4px; }

  .scan-bar { display:flex; align-items:center; gap:var(--space-2); padding:var(--space-2) 0; font-size:var(--text-xs); color:var(--text-muted); }
  .scan-text { flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-family:var(--font-mono); }
  .scan-count { color:var(--accent); font-weight:600; }
  .spinner { width:14px; height:14px; border:2px solid var(--border); border-top-color:var(--accent); border-radius:50%; animation:spin 0.8s linear infinite; flex-shrink:0; }
  @keyframes spin { to { transform:rotate(360deg); } }

  .empty { display:flex; flex-direction:column; align-items:center; gap:var(--space-2); padding:var(--space-6) 0; color:var(--text-muted); font-size:var(--text-sm); text-align:center; }
  .sub { font-size:var(--text-xs); color:var(--text-faint); }
  code { background:var(--bg-subtle); padding:1px 4px; border-radius:3px; font-family:var(--font-mono); font-size:var(--text-xs); }

  .source-list { max-height:45vh; overflow:auto; display:flex; flex-direction:column; gap:var(--space-2); }
  .project-header { display:flex; align-items:center; gap:var(--space-1); padding:var(--space-1) 0; }
  .project-path { font-family:var(--font-mono); font-size:var(--text-xs); color:var(--text-muted); }
  .source-row { display:flex; align-items:center; gap:var(--space-2); padding:var(--space-2) var(--space-3); background:var(--bg-subtle); border:1px solid var(--border-subtle); border-radius:var(--radius-sm); }
  .editor { font-size:var(--text-sm); font-weight:500; color:var(--text); }
  .count { flex:1; font-size:var(--text-xs); color:var(--text-faint); }
  .imported { display:flex; align-items:center; gap:4px; font-size:var(--text-xs); color:var(--success); }
  .sm { padding:3px 10px; font-size:var(--text-xs); }

  .actions { display:flex; gap:var(--space-2); justify-content:center; padding-top:var(--space-2); }
</style>
