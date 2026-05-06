<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Events } from "@wailsio/runtime";
  import Icon from "./Icon.svelte";
  import {
    debugFiles,
    loadDebugFiles,
    removeStaleDebugFiles,
    type DebugFileEntry,
  } from "./settings-store";
  import {
    pickWorkspaceFolder,
    refreshWorkspace,
    refreshTargets,
    openDebugFile,
  } from "./store";
  import * as DetectService from "../../bindings/github.com/jp/DelveUI/internal/detect/service";

  export let visible = false;
  export let onDone: () => void = () => {};

  // Welcome page is structured around the same primary action VS Code uses:
  // "Open a folder". The system-wide auto-detect is still available but is
  // explicitly secondary — most users want to open the folder they're working
  // on, not crawl their filesystem on first launch.

  type Source = {
    editor: string;
    projectPath: string;
    configPath: string;
    configCount: number;
    configs: any[];
    imported: boolean;
    importing: boolean;
  };

  // --- recents (primary view) ---
  let recents: DebugFileEntry[] = [];
  let loadingRecents = true;

  async function refreshRecents() {
    await loadDebugFiles();
    recents = ($debugFiles ?? [])
      .slice()
      .sort((a, b) => {
        if (!!a.stale !== !!b.stale) return a.stale ? 1 : -1;
        const at = new Date(a.lastUsed ?? a.addedAt).getTime() || 0;
        const bt = new Date(b.lastUsed ?? b.addedAt).getTime() || 0;
        return bt - at;
      });
    loadingRecents = false;
  }

  // --- import path (secondary view) ---
  let importOpen = false;
  let sources: Source[] = [];
  let scanning = false;
  let scanStatus = "";
  let scanDone = false;

  const editorIcons: Record<string, string> = {
    "Zed": "devicon:zed",
    "VS Code": "devicon:vscode",
    "GoLand": "devicon:goland",
  };

  onMount(() => {
    if (visible) refreshRecents();
  });

  $: if (visible) refreshRecents();

  // Streaming scan events from the backend.
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

  async function startImport() {
    importOpen = true;
    if (!scanDone && !scanning) await scan();
  }

  async function scan() {
    scanning = true;
    scanDone = false;
    sources = [];
    scanStatus = "Starting scan…";
    try {
      const raw = (await DetectService.Scan()) as any as Source[];
      for (const s of raw ?? []) {
        if (!sources.find(x => x.configPath === s.configPath)) {
          sources = [...sources, { ...s, imported: false, importing: false }];
        }
      }
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
      await refreshRecents();
      await refreshWorkspace();
    } catch (e) { console.error(e); }
    finally { s.importing = false; sources = sources; }
  }

  async function importAll() {
    for (const s of sources) { if (!s.imported) await importSource(s); }
  }

  async function openFolder() {
    await pickWorkspaceFolder();
    refreshTargets().catch(() => {});
    onDone();
  }

  async function openRecent(e: DebugFileEntry) {
    if (e.stale) return;
    await openDebugFile(e.path);
    refreshTargets().catch(() => {});
    onDone();
  }

  async function scanFolder() {
    try {
      const result = (await DetectService.PickAndScanFolder()) as any;
      if (!result?.projectPath) return;
      const found = (result.editorConfigs ?? []) as Source[];
      if (found.length === 0) {
        const { showInfo } = await import("./toast");
        showInfo(
          "No debug configs found",
          "You can still open this folder via Open Folder — DelveUI will auto-discover Go run targets.",
        );
        return;
      }
      const fresh: Source[] = [];
      for (const s of found) {
        if (sources.find((x) => x.configPath === s.configPath)) continue;
        let imported = false;
        try { imported = (await DetectService.IsImported(s.configPath)) as boolean; } catch {}
        fresh.push({ ...s, imported, importing: false });
      }
      if (fresh.length > 0) {
        sources = [...sources, ...fresh];
        scanDone = true;
      }
      importOpen = true;
    } catch (e) {
      const { showError } = await import("./toast");
      showError("Folder scan failed", String((e as any)?.message ?? e));
    }
  }

  async function cleanStale() {
    const n = await removeStaleDebugFiles();
    await refreshRecents();
    if (n > 0) {
      const { showInfo } = await import("./toast");
      showInfo(`Removed ${n} missing project${n === 1 ? "" : "s"}`, "");
    }
  }

  function shortPath(p: string) { return p.replace(/^\/Users\/[^/]+/, "~"); }

  function fmtRecency(d?: string): string {
    if (!d) return "";
    const ts = new Date(d).getTime();
    if (!ts) return "";
    const sec = Math.max(0, Math.floor((Date.now() - ts) / 1000));
    if (sec < 60) return "just now";
    if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
    if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
    if (sec < 86400 * 30) return `${Math.floor(sec / 86400)}d ago`;
    return new Date(ts).toLocaleDateString();
  }

  // ---- import view groups, used only when the user opens the import drawer.
  type Workspace = {
    path: string;
    sources: Source[];
    totalConfigs: number;
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

  $: hasStale = recents.some((e) => e.stale);
</script>

{#if visible}
  <div class="welcome">
    <div class="container">
      <div class="header">
        <Icon icon="solar:code-2-bold-duotone" size={36} color="var(--accent)" />
        <div>
          <h1>Welcome to DelveUI</h1>
          <p class="subtitle">Open a folder to start debugging Go.</p>
        </div>
      </div>

      <!-- Primary CTAs: lead with Open Folder, the action 95% of users want. -->
      <div class="primary-actions">
        <button class="cta primary" on:click={openFolder}>
          <Icon icon="solar:folder-with-files-bold" size={16} />
          <span class="cta-label">
            <span class="cta-title">Open Folder…</span>
            <span class="cta-sub">Pick the project you're working on</span>
          </span>
        </button>
      </div>

      {#if loadingRecents}
        <div class="recents-empty">
          <div class="spinner"></div>
          <span>Loading recent projects…</span>
        </div>
      {:else if recents.length > 0}
        <div class="section">
          <div class="section-head">
            <span class="section-title">Recent</span>
            {#if hasStale}
              <button class="link" on:click={cleanStale}>Remove missing</button>
            {/if}
          </div>
          <div class="recent-list">
            {#each recents.slice(0, 8) as e (e.id)}
              <button
                class="recent"
                class:stale={e.stale}
                title={e.stale ? `Folder missing: ${e.path}` : e.path}
                on:click={() => openRecent(e)}
                disabled={e.stale}
              >
                <Icon icon="solar:folder-open-bold" size={14} color={e.stale ? "var(--text-faint)" : "var(--accent)"} />
                <div class="recent-info">
                  <div class="recent-title">
                    <span class="recent-name">{e.label}</span>
                    {#if e.stale}<span class="missing">missing</span>{/if}
                    {#if e.configs?.length}
                      <span class="recent-cfg">{e.configs.length} cfg{e.configs.length !== 1 ? "s" : ""}</span>
                    {/if}
                  </div>
                  <div class="recent-path">{shortPath(e.path)}</div>
                </div>
                <span class="recent-time">{fmtRecency(e.lastUsed ?? e.addedAt)}</span>
              </button>
            {/each}
          </div>
        </div>
      {/if}

      <!-- Secondary: editor import. Hidden behind a toggle so it doesn't
           dominate the page on first run. -->
      <div class="section secondary">
        <button class="section-toggle" on:click={() => importOpen = !importOpen}>
          <Icon icon={importOpen ? "solar:alt-arrow-down-linear" : "solar:alt-arrow-right-linear"} size={11} />
          <Icon icon="solar:magnifer-linear" size={12} color="var(--text-faint)" />
          <span>Have launch.json files in another editor? Import them</span>
          {#if sources.length > 0}
            <span class="badge">{sources.length}</span>
          {/if}
        </button>
        {#if importOpen}
          <div class="import-body">
            <div class="import-actions">
              <button class="btn outlined" on:click={scan} disabled={scanning}>
                <Icon icon="solar:refresh-linear" size={12} />
                {scanning ? "Scanning…" : (scanDone ? "Rescan" : "Scan home dir")}
              </button>
              <button class="btn outlined" on:click={scanFolder}>
                <Icon icon="solar:folder-with-files-bold" size={12} />
                Scan a folder…
              </button>
            </div>

            {#if scanning}
              <div class="scan-bar">
                <div class="spinner"></div>
                <span class="scan-text">{scanStatus}</span>
                <span class="scan-count">{sources.length} found</span>
              </div>
            {:else if scanDone && sources.length === 0}
              <div class="empty">
                <Icon icon="solar:inbox-bold" size={20} color="var(--text-faint)" />
                <span>No editor debug configs found.</span>
                <span class="sub">Most projects don't need this. Use <strong>Open Folder…</strong> instead.</span>
              </div>
            {/if}

            {#if sources.length > 0}
              <div class="source-list">
                {#each workspaces as w (w.path)}
                  {@const imported = workspaceImported(w)}
                  {@const importing = workspaceImporting(w)}
                  <div class="workspace" class:expanded={expandedPaths.has(w.path)}>
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <!-- svelte-ignore a11y-no-static-element-interactions -->
                    <div class="workspace-row" on:click={() => toggleExpanded(w.path)} role="button" tabindex="0">
                      <span class="chev"><Icon icon="solar:alt-arrow-right-linear" size={10} /></span>
                      <Icon icon="solar:folder-bold" size={14} color="var(--text-muted)" />
                      <span class="workspace-name">{workspaceName(w.path)}</span>
                      <span class="workspace-path">{shortPath(w.path)}</span>
                      <div class="editor-stack">
                        {#each w.sources as s}
                          <Icon icon={editorIcons[s.editor] ?? "solar:code-bold"} size={14} />
                        {/each}
                      </div>
                      <span class="count">{w.totalConfigs} config{w.totalConfigs !== 1 ? "s" : ""}</span>
                      {#if imported}
                        <span class="imported"><Icon icon="solar:check-circle-bold" size={13} color="var(--success)" /> Done</span>
                      {:else}
                        <button class="btn primary sm" on:click|stopPropagation={() => importWorkspace(w)} disabled={importing}>
                          {importing ? "…" : "Import"}
                        </button>
                      {/if}
                    </div>
                    {#if expandedPaths.has(w.path)}
                      <div class="workspace-detail">
                        {#each w.sources as s}
                          <div class="source-row">
                            <Icon icon={editorIcons[s.editor] ?? "solar:code-bold"} size={14} />
                            <span class="editor">{s.editor}</span>
                            <span class="count">{s.configCount} config{s.configCount !== 1 ? "s" : ""}</span>
                            {#if s.imported}
                              <span class="imported"><Icon icon="solar:check-circle-bold" size={12} color="var(--success)" /> Done</span>
                            {:else}
                              <button class="btn sm" on:click|stopPropagation={() => importSource(s)} disabled={s.importing}>
                                {s.importing ? "…" : "Import"}
                              </button>
                            {/if}
                          </div>
                        {/each}
                      </div>
                    {/if}
                  </div>
                {/each}
                {#if sources.length > 1 && !sources.every(s => s.imported)}
                  <button class="btn outlined" style="margin-top:8px;align-self:flex-end" on:click={importAll}>
                    Import all
                  </button>
                {/if}
              </div>
            {/if}
          </div>
        {/if}
      </div>

      <div class="footer">
        <button class="link" on:click={onDone}>Skip</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .welcome { position:fixed; inset:0; z-index:700; background:var(--bg); display:flex; align-items:flex-start; justify-content:center; overflow:auto; }
  .container { width:100%; max-width:620px; padding:48px 24px; display:flex; flex-direction:column; gap:24px; }

  .header { display:flex; align-items:center; gap:14px; }
  h1 { font-size:22px; font-weight:700; color:var(--text); margin:0; }
  .subtitle { color:var(--text-muted); font-size:14px; margin:0; }

  .primary-actions { display:flex; flex-direction:column; gap:8px; }
  .cta {
    display:flex; align-items:center; gap:14px;
    width:100%; padding:14px 16px;
    background:var(--accent); color:#fff;
    border:1px solid var(--accent);
    border-radius:10px;
    cursor:pointer;
    text-align:left;
    transition:background 80ms ease, transform 80ms ease;
  }
  .cta:hover { background:#5ea6ff; }
  .cta-label { display:flex; flex-direction:column; gap:2px; }
  .cta-title { font-size:15px; font-weight:600; }
  .cta-sub { font-size:12px; opacity:0.85; }

  .recents-empty {
    display:flex; align-items:center; gap:10px;
    color:var(--text-muted); font-size:13px;
    padding:8px 4px;
  }

  .section { display:flex; flex-direction:column; gap:8px; }
  .section.secondary { margin-top:4px; }
  .section-head {
    display:flex; align-items:center; gap:8px;
    color:var(--text-faint); font-size:11px; font-weight:600;
    text-transform:uppercase; letter-spacing:0.6px;
    padding:0 4px;
  }
  .section-title { flex:1; }
  .link {
    background:transparent; border:0; cursor:pointer;
    color:var(--text-faint); font-size:11px;
    padding:0; text-decoration:underline;
  }
  .link:hover { color:var(--text-muted); }

  .recent-list { display:flex; flex-direction:column; gap:4px; }
  .recent {
    display:flex; align-items:center; gap:12px;
    padding:10px 12px;
    background:var(--bg-elevated);
    border:1px solid var(--border-subtle);
    border-radius:8px;
    cursor:pointer; text-align:left; color:var(--text); font:inherit;
    transition:background 80ms ease, border-color 80ms ease;
  }
  .recent:hover:not(:disabled) {
    border-color:var(--accent);
  }
  .recent:disabled { opacity:0.55; cursor:default; }
  .recent.stale { background:var(--bg-subtle); }
  .recent-info { flex:1; min-width:0; }
  .recent-title { display:flex; align-items:center; gap:8px; }
  .recent-name { font-weight:600; font-size:13px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .missing {
    color:var(--danger); font-size:9px; font-family:var(--font-mono);
    padding:0 4px; border:1px solid var(--border-subtle); border-radius:3px;
  }
  .recent-cfg { font-size:10px; color:var(--text-faint); font-family:var(--font-mono); }
  .recent-path { font-family:var(--font-mono); font-size:11px; color:var(--text-faint); margin-top:2px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .recent-time { font-size:10px; color:var(--text-faint); font-family:var(--font-mono); flex-shrink:0; }

  .section-toggle {
    display:flex; align-items:center; gap:8px;
    background:transparent; border:1px dashed var(--border-subtle); border-radius:8px;
    padding:10px 12px;
    cursor:pointer; text-align:left; font:inherit;
    color:var(--text-muted); font-size:13px;
  }
  .section-toggle:hover { border-color:var(--border); color:var(--text); }
  .section-toggle .badge {
    margin-left:auto;
    background:var(--accent); color:#fff;
    font-size:9px; padding:1px 6px; border-radius:8px;
  }

  .import-body {
    display:flex; flex-direction:column; gap:10px;
    padding:12px; margin-top:-4px;
    background:var(--bg-subtle);
    border:1px solid var(--border-subtle);
    border-top:0;
    border-radius:0 0 8px 8px;
  }
  .import-actions { display:flex; gap:8px; }

  .scan-bar { display:flex; align-items:center; gap:8px; font-size:12px; color:var(--text-muted); }
  .scan-text { flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-family:var(--font-mono); }
  .scan-count { color:var(--accent); font-weight:600; }
  .spinner { width:14px; height:14px; border:2px solid var(--border); border-top-color:var(--accent); border-radius:50%; animation:spin 0.8s linear infinite; flex-shrink:0; }
  @keyframes spin { to { transform:rotate(360deg); } }

  .empty { display:flex; flex-direction:column; align-items:center; gap:6px; padding:18px 12px; color:var(--text-muted); font-size:12px; text-align:center; }
  .sub { font-size:11px; color:var(--text-faint); }

  .source-list { max-height:35vh; overflow:auto; display:flex; flex-direction:column; gap:6px; }

  .workspace { display:flex; flex-direction:column; }
  .workspace-row {
    display:flex; align-items:center; gap:8px;
    padding:6px 10px;
    background:var(--bg); border:1px solid var(--border-subtle);
    border-radius:6px; cursor:pointer;
  }
  .workspace-row:hover { background:rgba(255,255,255,0.04); }
  .chev { display:inline-flex; transition:transform 120ms ease; color:var(--text-faint); }
  .workspace.expanded .chev { transform:rotate(90deg); }
  .workspace-name { font-size:12px; font-weight:600; color:var(--text); }
  .workspace-path { font-family:var(--font-mono); font-size:10px; color:var(--text-faint); flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .editor-stack { display:inline-flex; align-items:center; gap:4px; flex-shrink:0; }
  .workspace-detail { margin:4px 0 0 24px; padding:8px; background:var(--bg); border:1px solid var(--border-subtle); border-radius:6px; display:flex; flex-direction:column; gap:2px; }
  .source-row { display:flex; align-items:center; gap:8px; padding:4px 8px; }
  .editor { font-size:12px; font-weight:500; color:var(--text); }
  .count { font-size:11px; color:var(--text-faint); flex-shrink:0; }
  .imported { display:flex; align-items:center; gap:4px; font-size:11px; color:var(--success); flex-shrink:0; }
  .sm { padding:3px 10px; font-size:11px; }

  .footer { display:flex; justify-content:center; padding-top:6px; }
</style>
