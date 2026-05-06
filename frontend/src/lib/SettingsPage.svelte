<script lang="ts">
  import { onMount, onDestroy, tick } from "svelte";
  import Icon from "./Icon.svelte";
  import {
    appSettings,
    debugFiles,
    loadSettings,
    saveSettings,
    loadDebugFiles,
    addDebugFile,
    removeDebugFile,
    removeStaleDebugFiles,
    reloadDebugFile,
    type AppSettings,
    type DebugFileEntry,
  } from "./settings-store";
  import {
    themeList,
    currentThemeName,
    refreshThemeList,
    setTheme,
    loadTheme,
    previewThemeByName,
    revertThemePreview,
  } from "./theme-engine";
  import { pickDebugFile, refreshWorkspace, openDebugFile, workspace } from "./store";
  import * as WorkspaceService from "../../bindings/github.com/jp/DelveUI/internal/services/workspaceservice";
  import { showInfo, showError } from "./toast";
  import * as UpdateService from "../../bindings/github.com/jp/DelveUI/internal/updater/service";

  let updateInfo: any = null;
  let checking = false;
  let appInfo: any = null;
  type UpdateState = "idle" | "downloading" | "ready" | "error";
  let updateState: UpdateState = "idle";
  let dlBytes = 0;
  let dlTotal = 0;
  let dlError = "";

  async function checkUpdates() {
    checking = true;
    updateInfo = null;
    try {
      const info = await UpdateService.CheckForUpdate() as any;
      updateInfo = info;
      if (!info?.available) {
        showInfo("Up to date", `You're on the latest version (${info?.currentVersion ?? "?"})`);
      }
    } catch (e: any) {
      showError("Update check failed", String(e?.message ?? e));
    } finally {
      checking = false;
    }
  }

  async function loadAppInfo() {
    try { appInfo = await UpdateService.AppInfo() as any; } catch {}
  }

  async function downloadUpdate() {
    updateState = "downloading";
    dlBytes = 0;
    dlTotal = 0;
    dlError = "";
    try {
      await UpdateService.DownloadUpdate();
      // progress handler (below) flips state to "ready" on done.
    } catch (e: any) {
      updateState = "error";
      dlError = String(e?.message ?? e);
    }
  }

  async function relaunchNewVersion() {
    try {
      await UpdateService.ApplyUpdate();
      // App will quit itself; nothing else to do.
    } catch (e: any) {
      updateState = "error";
      dlError = String(e?.message ?? e);
    }
  }

  async function openReleaseInBrowser() {
    try {
      await (UpdateService as any).OpenReleasePage?.();
    } catch {}
  }

  // Subscribe to backend download progress once; update reactive state.
  let progressUnsub: (() => void) | null = null;
  onMount(async () => {
    const { Events } = await import("@wailsio/runtime");
    progressUnsub = Events.On("update:progress", (ev: any) => {
      const d = ev?.data ?? ev ?? {};
      if (d.error) { updateState = "error"; dlError = d.error; return; }
      if (typeof d.downloaded === "number") dlBytes = d.downloaded;
      if (typeof d.total === "number" && d.total > 0) dlTotal = d.total;
      if (d.done) updateState = "ready";
    });
  });

  onDestroy(() => {
    if (progressUnsub) progressUnsub();
  });

  function fmtBytes(n: number): string {
    if (n < 1024) return n + " B";
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
    return (n / (1024 * 1024)).toFixed(1) + " MB";
  }

  $: dlPercent = dlTotal > 0 ? Math.min(100, Math.round((dlBytes / dlTotal) * 100)) : 0;
  import * as ThemeService from "../../bindings/github.com/jp/DelveUI/internal/themes/service";
  import * as SettingsServiceBinding from "../../bindings/github.com/jp/DelveUI/internal/settings/service";
  import * as DebugFilesStoreBinding from "../../bindings/github.com/jp/DelveUI/internal/debugfiles/store";

  export let open = false;
  export let onOpenImport: () => void = () => {};

  type Tab = "appearance" | "terminal" | "debugfiles" | "general";
  const allTabs: Tab[] = ["appearance", "terminal", "debugfiles", "general"];
  const tabIcons: Record<Tab, string> = {
    appearance: "solar:palette-bold",
    terminal: "solar:monitor-bold",
    debugfiles: "solar:document-bold",
    general: "solar:settings-bold",
  };

  // Settings search
  let searchQuery = "";
  const tabLabels: Record<Tab, string> = {
    appearance: "Appearance",
    terminal: "Terminal",
    debugfiles: "Debug Files",
    general: "General",
  };

  let tab: Tab = "appearance";
  let settings: AppSettings = {
    theme: "One Dark", terminalTheme: "follow", vimMode: false,
    uiFontSize: 13, bufferFontSize: 13, termFontSize: 12, lineHeight: "standard",
    dlvPath: "", restoreLastProject: true,
    leftPanels: [], rightPanels: [], defaultLeftTab: "", defaultRightTab: "",
  };
  let previewTheme: string | null = null;
  let committed = false;
  let sidebarEl: HTMLElement;
  let contentEl: HTMLElement;

  // Don't reactively derive settings — it resets inputs mid-drag.
  // Instead, load once on open.
  $: if (open) onOpen();

  async function onOpen() {
    await tick();
    await loadSettings();
    settings = { ...$appSettings };
    loadDebugFiles();
    refreshThemeList();
    loadAppInfo();
    focusSidebar();
  }

  function close() {
    if (previewTheme) {
      revertThemePreview();
      previewTheme = null;
    }
    open = false;
  }

  // --- Keyboard ---
  function onModalKey(e: KeyboardEvent) {
    if (e.key === "Escape") { e.preventDefault(); close(); return; }

    const mod = e.metaKey || e.ctrlKey;
    if (mod && e.key >= "1" && e.key <= "5") {
      e.preventDefault();
      tab = allTabs[parseInt(e.key) - 1] ?? tab;
      focusContent();
      return;
    }

    const inSidebar = sidebarEl?.contains(document.activeElement);

    if (inSidebar) {
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        const idx = allTabs.indexOf(tab);
        const next = e.key === "ArrowDown" ? Math.min(allTabs.length - 1, idx + 1) : Math.max(0, idx - 1);
        tab = allTabs[next];
        sidebarBtn(next)?.focus();
      } else if (e.key === "ArrowRight" || e.key === "Enter") {
        e.preventDefault();
        focusContent();
      }
    } else {
      if (e.key === "ArrowLeft" && !(e.target instanceof HTMLInputElement) && !(e.target instanceof HTMLTextAreaElement)) {
        e.preventDefault();
        focusSidebar();
      }
    }
  }

  function sidebarBtn(idx: number): HTMLButtonElement | null {
    return sidebarEl?.querySelectorAll<HTMLButtonElement>("button[data-tab]")?.[idx] ?? null;
  }

  function focusSidebar() {
    const idx = allTabs.indexOf(tab);
    requestAnimationFrame(() => sidebarBtn(idx)?.focus());
  }

  function focusContent() {
    requestAnimationFrame(() => {
      const el = contentEl?.querySelector<HTMLElement>("button, input, [role='option'], [tabindex='0']");
      el?.focus();
    });
  }

  // --- Theme ---
  async function selectTheme(name: string) {
    committed = true;
    previewTheme = null;
    settings.theme = name;
    await setTheme(name);
    await save();
    setTimeout(() => (committed = false), 150);
  }

  function previewT(name: string) {
    if (committed) return;
    previewTheme = name;
    previewThemeByName(name);
  }

  function revertPreview() {
    if (committed) return;
    if (previewTheme) {
      revertThemePreview();
      previewTheme = null;
    }
  }

  function themeKey(e: KeyboardEvent, name: string) {
    if (e.key === "Enter" || e.key === " ") { e.preventDefault(); selectTheme(name); }
  }

  // --- Settings ---
  async function save() { await saveSettings(settings); }

  function updateSetting<K extends keyof AppSettings>(key: K, value: AppSettings[K]) {
    settings = { ...settings, [key]: value };
    save();
    applyFontSettings(settings);
  }

  function applyFontSettings(s: AppSettings) {
    const root = document.documentElement;
    if (s.uiFontSize) root.style.setProperty("--text-md", s.uiFontSize + "px");
    if (s.bufferFontSize) root.style.setProperty("--text-sm", s.bufferFontSize + "px");
    if (s.termFontSize) root.style.setProperty("--text-term", s.termFontSize + "px");
    if (s.lineHeight) {
      const lh = s.lineHeight === "compact" ? "1.2" : s.lineHeight === "comfortable" ? "1.618" : "1.3";
      root.style.setProperty("--lh-standard", lh);
    }
  }

  // --- Debug files ---
  async function addFile() {
    await pickDebugFile();
    const ws = (await import("./store")).workspace;
    const unsub = ws.subscribe(async (w) => { if (w?.debugFile) { await addDebugFile(w.debugFile); unsub(); } });
  }
  async function switchToFile(entry: DebugFileEntry) { await openDebugFile(entry.path); await refreshWorkspace(); }
  async function installTheme() { try { const m = (await ThemeService.ImportFile("")) as any; if (m?.name) await refreshThemeList(); } catch {} }

  let confirmAction: (() => Promise<void>) | null = null;
  let confirmMessage = "";

  function requestReset(msg: string, action: () => Promise<void>) {
    confirmMessage = msg;
    confirmAction = action;
  }

  async function executeReset() {
    const action = confirmAction;
    confirmAction = null;
    confirmMessage = "";
    if (action) await action();
  }

  function cancelReset() {
    confirmAction = null;
    confirmMessage = "";
  }

  function resetSettings() {
    requestReset("Reset all settings to defaults?", async () => {
      try {
        await SettingsServiceBinding.Reset();
        localStorage.clear();
        showInfo("Settings reset", "All settings restored to defaults. Reloading…");
        setTimeout(() => location.reload(), 1200);
      } catch (e: any) {
        showError("Reset failed", String(e?.message ?? e));
      }
    });
  }

  function resetDebugFiles() {
    requestReset("Remove all saved debug files?", async () => {
      try {
        await DebugFilesStoreBinding.Clear();
        await WorkspaceService.ClearWorkspace();
        showInfo("Debug files cleared", "All saved debug files removed. Reloading…");
        setTimeout(() => location.reload(), 1200);
      } catch (e: any) {
        showError("Reset failed", String(e?.message ?? e));
      }
    });
  }

  function resetEverything() {
    requestReset("Reset everything? Settings, debug files, layout — all gone.", async () => {
      try {
        await SettingsServiceBinding.Reset();
        await DebugFilesStoreBinding.Clear();
        await WorkspaceService.ClearWorkspace();
        localStorage.clear();
        showInfo("Everything reset", "All data cleared. Reloading…");
        setTimeout(() => location.reload(), 1200);
      } catch (e: any) {
        showError("Reset failed", String(e?.message ?? e));
      }
    });
  }
</script>

{#if open}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="backdrop" role="presentation" on:click={close}></div>
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div class="modal" role="dialog" aria-modal="true" aria-label="Settings" on:keydown={onModalKey}>
    <div class="sidebar" bind:this={sidebarEl} role="tablist" aria-label="Settings sections">
      <div class="sidebar-title">Settings</div>
      {#each allTabs as t, i}
        <button
          data-tab={t}
          role="tab"
          aria-selected={tab === t}
          class:active={tab === t}
          on:click={() => { tab = t; focusContent(); }}
        >
          <Icon icon={tabIcons[t]} size={14} />
          {tabLabels[t]}
          <span class="tab-hint">⌘{i + 1}</span>
        </button>
      {/each}
    </div>

    <div class="content" bind:this={contentEl} role="tabpanel" aria-label={tabLabels[tab]}>
      <div class="content-header">
        <div class="search-box">
          <Icon icon="solar:magnifer-linear" size={13} color="var(--text-faint)" />
          <input class="search-input" bind:value={searchQuery} placeholder="Search settings…" />
        </div>
        <button class="btn icon" on:click={close} title="Close (Esc)">
          <Icon icon="solar:close-circle-linear" size={16} />
        </button>
      </div>

      {#if tab === "appearance"}
        <h2>Appearance</h2>

        <div class="card">
          <div class="card-header">
            <span class="card-title">Theme</span>
            <button class="btn outlined sm" on:click={installTheme}>
              <Icon icon="solar:upload-minimalistic-bold" size={11} /> Install
            </button>
          </div>
          <div class="theme-grid" role="listbox" aria-label="Theme list">
          {#each $themeList as t}
            <button
              class="theme-card"
              role="option"
              aria-selected={t.name === $currentThemeName}
              class:active={t.name === $currentThemeName && !previewTheme}
              class:previewing={t.name === previewTheme}
              on:mouseenter={() => previewT(t.name)}
              on:mouseleave={revertPreview}
              on:focus={() => previewT(t.name)}
              on:blur={revertPreview}
              on:click={() => selectTheme(t.name)}
              on:keydown={(e) => themeKey(e, t.name)}
            >
              <span class="badge badge-{t.appearance}">{t.appearance}</span>
              <span class="tname">{t.name}</span>
              <span class="tauthor">{t.author}</span>
            </button>
          {/each}
          </div>
        </div>

        <div class="card">
          <div class="card-header"><span class="card-title">Font Sizes</span></div>
          <div class="card-row">
            <span class="card-info"><span class="card-title">UI</span></span>
            <input type="range" min="10" max="18" bind:value={settings.uiFontSize} on:input={() => updateSetting("uiFontSize", settings.uiFontSize)} />
            <span class="val">{settings.uiFontSize}px</span>
          </div>
          <div class="card-row">
            <span class="card-info"><span class="card-title">Editor</span></span>
            <input type="range" min="10" max="22" bind:value={settings.bufferFontSize} on:input={() => updateSetting("bufferFontSize", settings.bufferFontSize)} />
            <span class="val">{settings.bufferFontSize}px</span>
          </div>
        </div>

        <div class="card">
          <div class="card-header"><span class="card-title">Line Height</span></div>
          <div class="card-row">
            <div class="btn-group" role="radiogroup" aria-label="Line height">
              {#each ["compact", "standard", "comfortable"] as lh}
                <button class="seg" class:active={settings.lineHeight === lh} on:click={() => updateSetting("lineHeight", lh)}>{lh}</button>
              {/each}
            </div>
          </div>
        </div>

      {:else if tab === "terminal"}
        <h2>Terminal</h2>
        <div class="card">
          <div class="card-header"><span class="card-title">Font Size</span></div>
          <div class="card-row">
            <input type="range" min="9" max="20" bind:value={settings.termFontSize} on:input={() => updateSetting("termFontSize", settings.termFontSize)} />
            <span class="val">{settings.termFontSize}px</span>
          </div>
        </div>
        <div class="card">
          <div class="card-header"><span class="card-title">Terminal Theme</span></div>
          <div class="card-row" style="flex-wrap:wrap">
            <button class="seg" class:active={settings.terminalTheme === "follow"} on:click={() => updateSetting("terminalTheme", "follow")}>Follow editor</button>
            {#each $themeList as t}
              <button class="seg" class:active={settings.terminalTheme === t.name} on:click={() => updateSetting("terminalTheme", t.name)}>{t.name}</button>
            {/each}
          </div>
        </div>

      {:else if tab === "debugfiles"}
        <h2>Projects</h2>
        <p class="desc">Folders registered as projects. The most recently used one auto-loads on launch when "Restore last project" is enabled.</p>
        <div class="row" style="margin-bottom: var(--space-3)">
          <button class="btn primary" on:click={addFile}>
            <Icon icon="solar:folder-with-files-bold" size={13} /> Open Folder…
          </button>
          <button class="btn outlined" on:click={() => { close(); onOpenImport(); }}>
            <Icon icon="solar:magnifer-bold" size={13} /> Import launch configs
          </button>
          {#if ($debugFiles ?? []).some((f) => f.stale)}
            <button class="btn outlined" on:click={async () => { const n = await removeStaleDebugFiles(); showInfo(`Removed ${n} missing project${n === 1 ? "" : "s"}`, ""); }}>
              <Icon icon="solar:eraser-linear" size={13} /> Clean up missing
            </button>
          {/if}
        </div>
        <div class="file-list">
          {#each $debugFiles as f}
            <div class="file-row" role="listitem" class:stale={f.stale}>
              <Icon icon="solar:folder-open-bold" size={14} color={f.stale ? "var(--text-faint)" : "var(--accent)"} />
              <div class="file-info">
                <div class="file-label">
                  {f.label}
                  {#if f.stale}<span class="missing-badge" title="Folder no longer exists">missing</span>{/if}
                </div>
                <div class="file-path">{f.path}</div>
                <div class="file-meta">{f.configs?.length ?? 0} configs</div>
              </div>
              {#if !f.stale}
                <button class="btn icon" title="Switch" on:click={() => switchToFile(f)}><Icon icon="solar:arrow-right-bold" size={13} /></button>
                <button class="btn icon" title="Reload configs" on:click={() => reloadDebugFile(f.id)}><Icon icon="solar:refresh-linear" size={13} /></button>
              {/if}
              <button class="btn icon danger" title="Remove" on:click={() => removeDebugFile(f.id)}><Icon icon="solar:trash-bin-minimalistic-linear" size={13} /></button>
            </div>
          {/each}
          {#if $debugFiles.length === 0}
            <div class="empty">No projects yet. Click <strong>Open Folder…</strong> to register one.</div>
          {/if}
        </div>

      {:else if tab === "general"}
        <h2>General</h2>
        <div class="card">
          <div class="card-row">
            <div class="card-info">
              <span class="card-title">Restore last project on launch</span>
              <span class="card-desc">When enabled, the most-recently-active project reopens automatically. Turn off to land on the welcome page each time.</span>
            </div>
            <label class="switch">
              <input
                type="checkbox"
                checked={settings.restoreLastProject !== false}
                on:change={(e) => updateSetting("restoreLastProject", e.currentTarget.checked)}
              />
              <span class="slider"></span>
            </label>
          </div>
          <div class="card-row">
            <div class="card-info">
              <span class="card-title">Vim Mode</span>
              <span class="card-desc">Use Vim keybindings in the source editor</span>
            </div>
            <label class="switch">
              <input type="checkbox" checked={settings.vimMode} on:change={(e) => updateSetting("vimMode", e.currentTarget.checked)} />
              <span class="slider"></span>
            </label>
          </div>
        </div>
        <div class="card">
          <div class="card-header"><span class="card-title">Keyboard Shortcuts</span></div>
          <div class="shortcuts">
            <div><span class="key">⌘⇧P</span> Command Palette</div>
            <div><span class="key">⌘O</span> Quick Open File</div>
            <div><span class="key">⌘K ⌘T</span> Theme Picker</div>
            <div><span class="key">⌘,</span> Settings</div>
            <div><span class="key">⌘F</span> Search in Source</div>
            <div><span class="key">Ctrl+`</span> Focus Terminal</div>
            <div><span class="key">F5</span> Continue</div>
            <div><span class="key">⇧F5</span> Stop</div>
            <div><span class="key">F10</span> Step Over</div>
            <div><span class="key">F11</span> Step In</div>
            <div><span class="key">⇧F11</span> Step Out</div>
          </div>
        </div>

        <div class="card">
          <div class="card-header"><span class="card-title">Updates</span></div>
          <div class="card-row">
            <div class="card-info">
              <span class="card-title">Current Version</span>
              <span class="card-desc">{appInfo?.version ?? "loading…"}</span>
            </div>
            <button class="btn outlined sm" on:click={checkUpdates} disabled={checking}>
              <Icon icon="solar:refresh-linear" size={11} />
              {checking ? "Checking…" : "Check for Updates"}
            </button>
          </div>
          {#if updateInfo?.available}
            <div class="card-row update-available">
              <Icon icon="solar:arrow-up-bold" size={14} color="var(--success)" />
              <div class="card-info">
                <span class="card-title" style="color:var(--success)">v{updateInfo.latestVersion} available</span>
                {#if updateInfo.releaseNotes}
                  <span class="card-desc">{updateInfo.releaseNotes.slice(0, 120)}{updateInfo.releaseNotes.length > 120 ? "…" : ""}</span>
                {/if}
              </div>

              {#if updateState === "idle"}
                <button class="btn primary sm" on:click={downloadUpdate}>
                  <Icon icon="solar:download-minimalistic-bold" size={11} /> Download
                </button>
              {:else if updateState === "downloading"}
                <div class="dl">
                  <div class="dl-bar">
                    <div class="dl-fill" style:width="{dlPercent}%"></div>
                  </div>
                  <div class="dl-meta">
                    {dlPercent}% — {fmtBytes(dlBytes)}{dlTotal ? " / " + fmtBytes(dlTotal) : ""}
                  </div>
                </div>
              {:else if updateState === "ready"}
                <button class="btn primary sm" on:click={relaunchNewVersion}>
                  <Icon icon="solar:restart-bold" size={11} /> Relaunch
                </button>
              {:else if updateState === "error"}
                <div class="dl-err">
                  <span class="err-text">{dlError || "Download failed"}</span>
                  <button class="btn outlined sm" on:click={openReleaseInBrowser}>
                    Open release page
                  </button>
                  <button class="btn sm" on:click={downloadUpdate}>Retry</button>
                </div>
              {/if}
            </div>
          {/if}
        </div>

        <div class="card">
          <div class="card-header"><span class="card-title">About</span></div>
          <div class="card-row">
            <span class="about-text">DelveUI — Delve debugger GUI for Go</span>
          </div>
          {#if appInfo}
            <div class="card-row">
              <span class="card-desc">Go {appInfo.go} · {appInfo.os}/{appInfo.arch}</span>
            </div>
          {/if}
        </div>

        <div class="field">
          <span class="field-label">Reset</span>
          <p class="desc">Clear all settings, debug files, and layout. App will restart in first-launch state.</p>
          <div class="row">
            <button class="btn danger" on:click={resetSettings}>
              <Icon icon="solar:restart-bold" size={13} /> Reset Settings
            </button>
            <button class="btn danger" on:click={resetDebugFiles}>
              <Icon icon="solar:trash-bin-minimalistic-bold" size={13} /> Clear Debug Files
            </button>
            <button class="btn danger" on:click={resetEverything}>
              <Icon icon="solar:shield-warning-bold" size={13} /> Reset Everything
            </button>
          </div>
        </div>
      {/if}
    </div>

    {#if confirmAction}
      <div class="confirm-overlay">
        <div class="confirm-box">
          <Icon icon="solar:shield-warning-bold" size={24} color="var(--danger)" />
          <p>{confirmMessage}</p>
          <div class="confirm-actions">
            <button class="btn" on:click={cancelReset}>Cancel</button>
            <button class="btn danger-fill" on:click={executeReset}>Confirm</button>
          </div>
        </div>
      </div>
    {/if}
  </div>
{/if}

<style>
  .backdrop { position:fixed; inset:0; background:rgba(0,0,0,0.45); z-index:800; }
  .modal { position:fixed; inset:40px; max-width:900px; margin:0 auto; background:var(--bg); border:1px solid var(--border); border-radius:var(--radius-md); display:flex; overflow:hidden; box-shadow:0 24px 64px rgba(0,0,0,0.5); z-index:801; }

  .sidebar { width:200px; background:var(--bg-subtle); border-right:1px solid var(--border); padding:var(--space-3); display:flex; flex-direction:column; gap:2px; flex-shrink:0; }
  .sidebar-title { font-size:var(--text-sm); font-weight:700; color:var(--text-muted); margin-bottom:var(--space-3); text-transform:uppercase; letter-spacing:0.5px; }
  .sidebar button { display:flex; align-items:center; gap:var(--space-2); background:transparent; border:0; color:var(--text-muted); padding:var(--space-2); border-radius:var(--radius-sm); font-size:var(--text-sm); cursor:pointer; text-align:left; outline:none; }
  .sidebar button:hover { background:var(--bg-elevated); color:var(--text); }
  .sidebar button:focus-visible { outline:2px solid var(--accent); outline-offset:-2px; color:var(--text); }
  .sidebar button.active { background:var(--accent-subtle); color:var(--text); }
  .tab-hint { margin-left:auto; font-family:var(--font-mono); font-size:9px; color:var(--text-faint); opacity:0.5; }

  .content { flex:1; overflow:auto; padding:var(--space-4) var(--space-5); }
  .content-header { display:flex; align-items:center; gap:var(--space-2); margin-bottom:var(--space-4); }
  .search-box { flex:1; display:flex; align-items:center; gap:var(--space-2); background:var(--bg-subtle); border:1px solid var(--border-subtle); border-radius:var(--radius-sm); padding:0 var(--space-2); height:28px; }
  .search-input { flex:1; background:transparent; border:0; color:var(--text); font-family:var(--font-ui); font-size:var(--text-sm); outline:none; }

  h2 { font-size:var(--text-lg); font-weight:600; color:var(--text); margin:0 0 var(--space-3); }
  h3 { font-size:var(--text-sm); font-weight:600; color:var(--text); margin:0 0 var(--space-2); }
  .desc { color:var(--text-muted); font-size:var(--text-sm); margin-bottom:var(--space-3); }

  /* Card system */
  .card { background:var(--bg-subtle); border:1px solid var(--border-subtle); border-radius:var(--radius-md); margin-bottom:var(--space-3); overflow:hidden; }
  .card-header { display:flex; align-items:center; justify-content:space-between; padding:var(--space-2) var(--space-3); border-bottom:1px solid var(--border-subtle); }
  .card-title { font-size:var(--text-sm); font-weight:600; color:var(--text); }
  .card-desc { font-size:var(--text-xs); color:var(--text-faint); }
  .card-row { display:flex; align-items:center; gap:var(--space-2); padding:var(--space-2) var(--space-3); }
  .card-row + .card-row { border-top:1px solid var(--border-subtle); }
  .card-info { flex:1; display:flex; flex-direction:column; gap:1px; min-width:0; }
  .sm { font-size:var(--text-xs); padding:3px 8px; height:auto; }

  .val { font-family:var(--font-mono); font-size:var(--text-sm); color:var(--text-faint); min-width:36px; text-align:right; }
  input[type="range"] { flex:1; max-width:200px; accent-color:var(--accent); height:4px; }

  /* Segmented buttons */
  .btn-group { display:flex; gap:0; }
  .seg { background:var(--bg); border:1px solid var(--border); color:var(--text-muted); padding:4px 12px; font-size:var(--text-xs); font-family:var(--font-ui); cursor:pointer; transition:all 80ms; }
  .seg:first-child { border-radius:var(--radius-sm) 0 0 var(--radius-sm); }
  .seg:last-child { border-radius:0 var(--radius-sm) var(--radius-sm) 0; }
  .seg:not(:first-child) { margin-left:-1px; }
  .seg:hover { color:var(--text); background:var(--bg-elevated); }
  .seg.active { background:var(--accent); color:#fff; border-color:var(--accent); z-index:1; }

  /* Toggle switch */
  .switch { position:relative; display:inline-block; width:36px; height:20px; flex-shrink:0; }
  .switch input { opacity:0; width:0; height:0; }
  .slider { position:absolute; cursor:pointer; inset:0; background:var(--border); border-radius:10px; transition:0.2s; }
  .slider:before { content:""; position:absolute; height:14px; width:14px; left:3px; bottom:3px; background:var(--text-faint); border-radius:50%; transition:0.2s; }
  .switch input:checked + .slider { background:var(--accent); }
  .switch input:checked + .slider:before { transform:translateX(16px); background:#fff; }

  .theme-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(150px,1fr)); gap:var(--space-2); padding:var(--space-2) var(--space-3); }
  .theme-card { display:flex; flex-direction:column; gap:2px; padding:var(--space-2); background:var(--bg); border:1px solid var(--border-subtle); border-radius:var(--radius-sm); text-align:left; cursor:pointer; transition:border-color 80ms; outline:none; }
  .theme-card:hover { border-color:var(--text-faint); }
  .theme-card:focus-visible { border-color:var(--accent); box-shadow:0 0 0 2px var(--accent-subtle); }
  .theme-card.active { border-color:var(--accent); background:var(--accent-subtle); }
  .theme-card.previewing { border-color:var(--warning); }
  .badge { font-size:9px; font-weight:600; text-transform:uppercase; padding:1px 5px; border-radius:3px; width:fit-content; }
  .badge-dark { background:var(--border); color:var(--text-muted); }
  .badge-light { background:#e5e5e5; color:#333; }
  .tname { font-size:var(--text-sm); font-weight:500; color:var(--text); }
  .tauthor { font-size:var(--text-xs); color:var(--text-faint); }

  .toggle { display:flex; align-items:center; gap:var(--space-2); cursor:pointer; font-size:var(--text-sm); color:var(--text); }
  .toggle input { accent-color:var(--accent); }
  .about-text { font-size:var(--text-sm); color:var(--text-muted); }
  .update-available { background:rgba(152,195,121,0.08); }

  .dl {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 180px;
  }
  .dl-bar {
    height: 6px;
    width: 100%;
    background: rgba(255, 255, 255, 0.08);
    border-radius: 3px;
    overflow: hidden;
  }
  .dl-fill {
    height: 100%;
    background: var(--accent);
    transition: width 120ms linear;
  }
  .dl-meta {
    font-size: 10px;
    color: var(--text-faint);
    font-family: var(--font-mono);
    text-align: right;
  }
  .dl-err {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 4px;
  }
  .dl-err .err-text {
    font-size: 11px;
    color: var(--danger);
    font-family: var(--font-mono);
    max-width: 220px;
    text-align: right;
  }

  .confirm-overlay {
    position:absolute; inset:0; background:rgba(0,0,0,0.5);
    display:flex; align-items:center; justify-content:center; z-index:10;
    border-radius:var(--radius-md);
  }
  .confirm-box {
    background:var(--bg-elevated); border:1px solid var(--border);
    border-radius:var(--radius-md); padding:var(--space-6);
    display:flex; flex-direction:column; align-items:center; gap:var(--space-3);
    max-width:360px; text-align:center;
    box-shadow:0 8px 32px rgba(0,0,0,0.4);
  }
  .confirm-box p { color:var(--text); font-size:var(--text-sm); margin:0; }
  .confirm-actions { display:flex; gap:var(--space-2); }
  .danger-fill { background:var(--danger); color:#fff; border-color:var(--danger); }
  .danger-fill:hover { background:#c95a62; }

  .dock-section { margin-bottom:var(--space-4); }
  .panel-list { display:flex; flex-direction:column; gap:var(--space-1); }

  .file-list { margin-top:var(--space-3); display:flex; flex-direction:column; gap:var(--space-1); }
  .file-row { display:flex; align-items:center; gap:var(--space-2); padding:var(--space-2); background:var(--bg-subtle); border:1px solid var(--border-subtle); border-radius:var(--radius-sm); outline:none; }
  .file-row:hover { background:var(--bg-elevated); }
  .file-row:focus-visible { outline:2px solid var(--accent); outline-offset:-1px; }
  .file-row.stale { opacity:0.55; }
  .file-info { flex:1; min-width:0; }
  .file-label { font-size:var(--text-sm); font-weight:600; color:var(--text); }
  .missing-badge { font-family:var(--font-mono); font-size:9px; color:var(--danger); background:var(--bg); border:1px solid var(--border-subtle); border-radius:3px; padding:0 4px; margin-left:6px; }
  .file-path { font-size:var(--text-xs); color:var(--text-faint); font-family:var(--font-mono); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .file-meta { font-size:var(--text-xs); color:var(--text-muted); }
  .empty { padding:var(--space-3); color:var(--text-faint); font-size:var(--text-sm); }

  .shortcuts { display:grid; grid-template-columns:1fr 1fr; gap:0; }
  .shortcuts div { padding:var(--space-1) var(--space-3); font-size:var(--text-sm); color:var(--text-muted); border-bottom:1px solid var(--border-subtle); }
  .key { display:inline-block; padding:1px 6px; background:var(--bg); border:1px solid var(--border); border-radius:var(--radius-sm); font-family:var(--font-mono); font-size:var(--text-xs); color:var(--text); margin-right:var(--space-2); min-width:48px; text-align:center; }
</style>
