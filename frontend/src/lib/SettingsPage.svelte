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
    type VimMapping,
    type VimMappingMode,
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
  import { pickWorkspaceFolder, refreshWorkspace, openDebugFile, workspace } from "./store";
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
  let adapterOutputUnsub: Map<string, (() => void)> = new Map();

  // ---- adapters ----
  interface AdapterInfo {
    language: string; label: string; description: string;
    installed: boolean; installCmd: string; installUrl?: string;
    installing: boolean; error?: string;
  }
  let adapters: AdapterInfo[] = [];
  let adapterOutput: Record<string, string> = {};

  async function loadAdapters() {
    try {
      const { List } = await import("../../bindings/github.com/jp/DelveUI/internal/adapter/service");
      adapters = (await List()) as AdapterInfo[];
    } catch {}
  }

  async function installAdapter(language: string) {
    try {
      const { Install } = await import("../../bindings/github.com/jp/DelveUI/internal/adapter/service");
      await Install(language);
    } catch {}
  }

  onMount(async () => {
    const { Events } = await import("@wailsio/runtime");
    progressUnsub = Events.On("update:progress", (ev: any) => {
      const d = ev?.data ?? ev ?? {};
      if (d.error) { updateState = "error"; dlError = d.error; return; }
      if (typeof d.downloaded === "number") dlBytes = d.downloaded;
      if (typeof d.total === "number" && d.total > 0) dlTotal = d.total;
      if (d.done) updateState = "ready";
    });
    adapterOutputUnsub.set("start", Events.On("adapter:install:start", (ev: any) => {
      const lang = ev?.data?.language ?? ev?.language ?? "";
      if (lang) adapterOutput[lang] = "";
      adapters = adapters.map(a => a.language === lang ? { ...a, installing: true, error: "" } : a);
    }));
    adapterOutputUnsub.set("output", Events.On("adapter:install:output", (ev: any) => {
      const lang = ev?.data?.language ?? ev?.language ?? "";
      const line = ev?.data?.line ?? ev?.line ?? "";
      if (lang) adapterOutput[lang] = (adapterOutput[lang] ?? "") + line + "\n";
      adapterOutput = adapterOutput; // trigger reactivity
    }));
    adapterOutputUnsub.set("done", Events.On("adapter:install:done", (ev: any) => {
      const lang = ev?.data?.language ?? ev?.language ?? "";
      if (lang) loadAdapters();
    }));
    adapterOutputUnsub.set("error", Events.On("adapter:install:error", (ev: any) => {
      const lang = ev?.data?.language ?? ev?.language ?? "";
      const err = ev?.data?.error ?? ev?.error ?? "";
      adapters = adapters.map(a => a.language === lang ? { ...a, installing: false, error: err as string } : a);
    }));
    loadAdapters();
  });

  onDestroy(() => {
    if (progressUnsub) progressUnsub();
    for (const unsub of adapterOutputUnsub.values()) unsub();
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

  type Tab = "appearance" | "terminal" | "debugfiles" | "vim" | "adapters" | "general";
  const allTabs: Tab[] = ["appearance", "terminal", "debugfiles", "vim", "adapters", "general"];
  const tabIcons: Record<Tab, string> = {
    appearance: "solar:palette-bold",
    terminal: "solar:monitor-bold",
    debugfiles: "solar:document-bold",
    vim: "solar:keyboard-bold",
    adapters: "solar:bug-minimalistic-bold",
    general: "solar:settings-bold",
  };

  // Settings search
  let searchQuery = "";
  const tabLabels: Record<Tab, string> = {
    appearance: "Appearance",
    terminal: "Terminal",
    debugfiles: "Debug Files",
    vim: "Vim",
    adapters: "Adapters",
    general: "General",
  };

  // Cross-tab search index. Each entry's `id` matches an `id` attribute on
  // a card in the template — that's how clicking a result jumps straight to
  // the relevant section instead of dumping the user on a tab.
  type SearchEntry = {
    id: string;
    label: string;
    description?: string;
    keywords?: string;
    tab: Tab;
  };
  const searchIndex: SearchEntry[] = [
    { id: "appearance-theme", label: "Theme", keywords: "color dark light appearance palette", tab: "appearance" },
    { id: "appearance-font-sizes", label: "Font Sizes", description: "UI and editor font size", keywords: "font size ui editor buffer text", tab: "appearance" },
    { id: "appearance-line-height", label: "Line Height", keywords: "spacing density compact comfortable standard", tab: "appearance" },
    { id: "terminal-font", label: "Terminal Font Size", keywords: "font size text", tab: "terminal" },
    { id: "terminal-theme", label: "Terminal Theme", keywords: "color follow editor", tab: "terminal" },
    { id: "debugfiles-projects", label: "Projects", description: "Registered folders", keywords: "folder workspace project import open debug launch json file", tab: "debugfiles" },
    { id: "general-toggles", label: "Restore last project on launch", description: "Reopen the most recent project at startup", keywords: "startup autoload reopen launch session", tab: "general" },
    { id: "vim-toggle", label: "Vim Mode", description: "Vim keybindings in the source editor", keywords: "vi keybindings editor modal", tab: "vim" },
    { id: "vim-mappings", label: "Vim Custom Mappings", description: "Define your own vim key mappings", keywords: "vim map remap keybinding lhs rhs normal visual insert", tab: "vim" },
    { id: "vim-cheatsheet", label: "Vim Cheat Sheet", description: "Reference of common vim bindings", keywords: "vim cheat reference motion editing visual search", tab: "vim" },
    { id: "adapters-list", label: "Debug Adapters", description: "Install debug adapters for different languages", keywords: "adapter debugger language install python node go delve debugpy", tab: "adapters" },
    { id: "general-shortcuts", label: "Keyboard Shortcuts", keywords: "keybindings hotkeys keys palette command", tab: "general" },
    { id: "general-updates", label: "Updates", description: "Check for and install new versions", keywords: "update version upgrade release download", tab: "general" },
    { id: "general-about", label: "About", keywords: "version info build", tab: "general" },
    { id: "general-reset", label: "Reset", description: "Clear settings, debug files, or factory reset", keywords: "clear factory reset settings everything", tab: "general" },
  ];

  function matchesQuery(q: string, hay: string): boolean {
    const needle = q.trim().toLowerCase();
    if (!needle) return true;
    const text = hay.toLowerCase();
    return needle.split(/\s+/).every((w) => text.includes(w));
  }

  $: searchActive = searchQuery.trim().length > 0;
  $: searchResults = searchActive
    ? searchIndex.filter((e) =>
        matchesQuery(
          searchQuery,
          [e.label, e.description ?? "", e.keywords ?? "", tabLabels[e.tab]].join(" "),
        ),
      )
    : [];

  function gotoSearchResult(e: SearchEntry) {
    tab = e.tab;
    searchQuery = "";
    // Wait for the tab content to render before scrolling.
    setTimeout(() => {
      const el = document.getElementById(e.id);
      if (!el) return;
      el.scrollIntoView({ behavior: "smooth", block: "start" });
      el.classList.add("flash-highlight");
      setTimeout(() => el.classList.remove("flash-highlight"), 1400);
    }, 30);
  }

  let tab: Tab = "appearance";
  let settings: AppSettings = {
    theme: "One Dark", terminalTheme: "follow", vimMode: false, vimMappings: [],
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
  }

  // --- Vim mappings ---
  const vimModes: VimMappingMode[] = ["normal", "visual", "insert"];

  function addVimMapping() {
    const next: VimMapping[] = [...(settings.vimMappings ?? []), { lhs: "", rhs: "", mode: "normal" }];
    updateSetting("vimMappings", next);
  }

  function removeVimMapping(idx: number) {
    const next = (settings.vimMappings ?? []).filter((_, i) => i !== idx);
    updateSetting("vimMappings", next);
  }

  function updateVimMapping(idx: number, patch: Partial<VimMapping>) {
    const next = (settings.vimMappings ?? []).map((m, i) => (i === idx ? { ...m, ...patch } : m));
    updateSetting("vimMappings", next);
  }

  function onVimMappingModeChange(idx: number, e: Event) {
    const v = (e.currentTarget as HTMLSelectElement).value as VimMappingMode;
    updateVimMapping(idx, { mode: v });
  }

  // Cheat-sheet of common bindings @replit/codemirror-vim ships out of the
  // box. This is a curated reference — not exhaustive — so users can see
  // what works without reading external docs.
  type CheatRow = { keys: string; desc: string };
  type CheatGroup = { title: string; rows: CheatRow[] };
  const vimCheatSheet: CheatGroup[] = [
    {
      title: "Motion",
      rows: [
        { keys: "h j k l", desc: "Left / Down / Up / Right" },
        { keys: "w / W", desc: "Next word / WORD start" },
        { keys: "b / B", desc: "Previous word / WORD start" },
        { keys: "e / E", desc: "End of word / WORD" },
        { keys: "0 / ^ / $", desc: "Line start / first non-blank / line end" },
        { keys: "gg / G", desc: "Top / bottom of file" },
        { keys: "{n}G", desc: "Jump to line n" },
        { keys: "% ", desc: "Match bracket" },
        { keys: "Ctrl-d / Ctrl-u", desc: "Half page down / up" },
      ],
    },
    {
      title: "Editing",
      rows: [
        { keys: "i / a", desc: "Insert before / after cursor" },
        { keys: "I / A", desc: "Insert at line start / end" },
        { keys: "o / O", desc: "Open line below / above" },
        { keys: "x", desc: "Delete character" },
        { keys: "dd / D", desc: "Delete line / to end of line" },
        { keys: "cc / C", desc: "Change line / to end of line" },
        { keys: "yy / Y", desc: "Yank line / to end of line" },
        { keys: "p / P", desc: "Paste after / before" },
        { keys: "u / Ctrl-r", desc: "Undo / Redo" },
        { keys: ".", desc: "Repeat last change" },
      ],
    },
    {
      title: "Operators + Motion",
      rows: [
        { keys: "dw / de", desc: "Delete word / to end of word" },
        { keys: "ciw / caw", desc: "Change inner word / a word" },
        { keys: "ci\" / ci'", desc: "Change inside quotes" },
        { keys: "ci( / cib", desc: "Change inside parens" },
        { keys: "ci{ / ciB", desc: "Change inside braces" },
        { keys: "yt,", desc: "Yank up to next comma" },
        { keys: "d$ / dG", desc: "Delete to end of line / file" },
      ],
    },
    {
      title: "Visual",
      rows: [
        { keys: "v", desc: "Character-wise visual" },
        { keys: "V", desc: "Line-wise visual" },
        { keys: "Ctrl-v", desc: "Block visual" },
        { keys: "o", desc: "Toggle selection anchor (in visual)" },
        { keys: "y / d / c", desc: "Yank / delete / change selection" },
        { keys: "> / <", desc: "Indent / dedent selection" },
      ],
    },
    {
      title: "Search & Replace",
      rows: [
        { keys: "/pattern", desc: "Search forward" },
        { keys: "?pattern", desc: "Search backward" },
        { keys: "n / N", desc: "Next / previous match" },
        { keys: "* / #", desc: "Search word under cursor fwd / back" },
        { keys: ":s/old/new/", desc: "Replace first on line" },
        { keys: ":%s/old/new/g", desc: "Replace everywhere" },
      ],
    },
    {
      title: "Marks & Jumps",
      rows: [
        { keys: "m{a-z}", desc: "Set mark" },
        { keys: "'{a-z}", desc: "Jump to mark line" },
        { keys: "`{a-z}", desc: "Jump to mark position" },
        { keys: "Ctrl-o / Ctrl-i", desc: "Older / newer jump" },
      ],
    },
    {
      title: "Ex commands",
      rows: [
        { keys: ":w", desc: "Save file" },
        { keys: ":wq", desc: "Save and (close) — same as :w here" },
        { keys: ":noh", desc: "Clear search highlight" },
        { keys: ":{n}", desc: "Jump to line n" },
      ],
    },
  ];

  // Cheat sheet filter state — matches against keys + desc + group title.
  let cheatQuery = "";
  $: filteredCheatSheet = (() => {
    const q = cheatQuery.trim().toLowerCase();
    if (!q) return vimCheatSheet;
    return vimCheatSheet
      .map((g) => ({
        ...g,
        rows: g.rows.filter((r) => {
          const hay = (g.title + " " + r.keys + " " + r.desc).toLowerCase();
          return q.split(/\s+/).every((w) => hay.includes(w));
        }),
      }))
      .filter((g) => g.rows.length > 0);
  })();

  // Curated suggestions users frequently want. Each click adds the row to
  // their custom mappings (deduped on lhs+mode).
  type MappingSuggestion = VimMapping & { note?: string };
  const mappingSuggestions: MappingSuggestion[] = [
    { lhs: "jk", rhs: "<Esc>", mode: "insert", note: "Quick exit from insert mode" },
    { lhs: "jj", rhs: "<Esc>", mode: "insert", note: "Alt: double-j exit" },
    { lhs: ";", rhs: ":", mode: "normal", note: "Avoid Shift to enter ex commands" },
    { lhs: "H", rhs: "^", mode: "normal", note: "Jump to first non-blank" },
    { lhs: "L", rhs: "$", mode: "normal", note: "Jump to end of line" },
    { lhs: "<Space>", rhs: ":", mode: "normal", note: "Spacebar opens ex prompt" },
    { lhs: "Y", rhs: "y$", mode: "normal", note: "Yank to end of line (vim default)" },
    { lhs: "<", rhs: "<gv", mode: "visual", note: "Re-select after dedent" },
    { lhs: ">", rhs: ">gv", mode: "visual", note: "Re-select after indent" },
  ];

  function addSuggestion(s: MappingSuggestion) {
    const cur = settings.vimMappings ?? [];
    const exists = cur.some((m) => m.lhs === s.lhs && m.mode === s.mode);
    if (exists) return;
    updateSetting("vimMappings", [...cur, { lhs: s.lhs, rhs: s.rhs, mode: s.mode }]);
  }

  function isSuggestionApplied(s: MappingSuggestion): boolean {
    return (settings.vimMappings ?? []).some((m) => m.lhs === s.lhs && m.mode === s.mode);
  }

  // --- Debug files ---
  async function addFolder() {
    await pickWorkspaceFolder();
    await refreshWorkspace();
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

      {#if searchActive}
        <h2>Search results</h2>
        {#if searchResults.length === 0}
          <div class="search-empty">
            No settings match &ldquo;{searchQuery}&rdquo;.
          </div>
        {:else}
          <div class="search-results">
            {#each searchResults as r (r.id)}
              <button class="search-result" on:click={() => gotoSearchResult(r)}>
                <Icon icon={tabIcons[r.tab]} size={13} color="var(--text-faint)" />
                <div class="sr-body">
                  <div class="sr-title">{r.label}</div>
                  {#if r.description}
                    <div class="sr-desc">{r.description}</div>
                  {/if}
                </div>
                <span class="sr-tab">{tabLabels[r.tab]}</span>
                <Icon icon="solar:alt-arrow-right-linear" size={11} color="var(--text-faint)" />
              </button>
            {/each}
          </div>
        {/if}
      {:else if tab === "appearance"}
        <h2>Appearance</h2>

        <div class="card" id="appearance-theme">
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

        <div class="card" id="appearance-font-sizes">
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

        <div class="card" id="appearance-line-height">
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
        <div class="card" id="terminal-font">
          <div class="card-header"><span class="card-title">Font Size</span></div>
          <div class="card-row">
            <input type="range" min="9" max="20" bind:value={settings.termFontSize} on:input={() => updateSetting("termFontSize", settings.termFontSize)} />
            <span class="val">{settings.termFontSize}px</span>
          </div>
        </div>
        <div class="card" id="terminal-theme">
          <div class="card-header"><span class="card-title">Terminal Theme</span></div>
          <div class="card-row" style="flex-wrap:wrap">
            <button class="seg" class:active={settings.terminalTheme === "follow"} on:click={() => updateSetting("terminalTheme", "follow")}>Follow editor</button>
            {#each $themeList as t}
              <button class="seg" class:active={settings.terminalTheme === t.name} on:click={() => updateSetting("terminalTheme", t.name)}>{t.name}</button>
            {/each}
          </div>
        </div>

      {:else if tab === "debugfiles"}
        <h2 id="debugfiles-projects">Projects</h2>
        <p class="desc">Folders registered as projects. The most recently used one auto-loads on launch when "Restore last project" is enabled.</p>
        <div class="row" style="margin-bottom: var(--space-3)">
          <button class="btn primary" on:click={addFolder}>
            <Icon icon="solar:folder-with-files-bold" size={13} /> Open folder…
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

      {:else if tab === "vim"}
        <h2>Vim</h2>
        <div class="card" id="vim-toggle">
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

        <div class="card" id="vim-mappings" class:disabled-card={!settings.vimMode}>
          <div class="card-header vim-map-header">
            <div class="card-info">
              <span class="card-title">Custom Mappings</span>
              <span class="card-desc">Map a key sequence (lhs) to another (rhs) for the chosen mode. Saved instantly; takes effect on the next keypress.</span>
            </div>
            <button class="btn outlined sm" on:click={addVimMapping} disabled={!settings.vimMode}>
              <Icon icon="solar:add-circle-linear" size={11} /> Add
            </button>
          </div>

          <div class="vim-suggest">
            <div class="vim-suggest-label">Suggestions</div>
            <div class="vim-suggest-chips">
              {#each mappingSuggestions as s}
                {@const applied = isSuggestionApplied(s)}
                <button
                  class="vim-suggest-chip"
                  class:applied
                  title={s.note ?? ""}
                  disabled={!settings.vimMode || applied}
                  on:click={() => addSuggestion(s)}
                >
                  <span class="vim-suggest-mode">{s.mode}</span>
                  <span class="vim-suggest-keys"><span class="key">{s.lhs}</span>→<span class="key">{s.rhs}</span></span>
                  {#if applied}<Icon icon="solar:check-circle-bold" size={10} color="var(--success)" />{/if}
                </button>
              {/each}
            </div>
          </div>

          {#if (settings.vimMappings ?? []).length === 0}
            <div class="empty">No custom mappings yet — pick a suggestion above or click Add.</div>
          {:else}
            <div class="vim-map-table">
              <div class="vim-map-row vim-map-head">
                <span>Mode</span>
                <span>From (lhs)</span>
                <span>To (rhs)</span>
                <span></span>
              </div>
              {#each settings.vimMappings as m, i (i)}
                <div class="vim-map-row">
                  <select
                    value={m.mode}
                    on:change={(e) => onVimMappingModeChange(i, e)}
                  >
                    {#each vimModes as mode}
                      <option value={mode}>{mode}</option>
                    {/each}
                  </select>
                  <input
                    type="text"
                    spellcheck="false"
                    placeholder="e.g. jk"
                    value={m.lhs}
                    on:change={(e) => updateVimMapping(i, { lhs: e.currentTarget.value })}
                  />
                  <input
                    type="text"
                    spellcheck="false"
                    placeholder={'e.g. <Esc>'}
                    value={m.rhs}
                    on:change={(e) => updateVimMapping(i, { rhs: e.currentTarget.value })}
                  />
                  <button class="btn icon danger" title="Remove" on:click={() => removeVimMapping(i)}>
                    <Icon icon="solar:trash-bin-minimalistic-linear" size={13} />
                  </button>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <div class="card" id="vim-cheatsheet">
          <div class="card-header vim-cheat-header">
            <span class="card-title">Cheat Sheet</span>
            <div class="vim-cheat-search">
              <Icon icon="solar:magnifer-linear" size={12} color="var(--text-faint)" />
              <input
                type="text"
                spellcheck="false"
                placeholder="Filter… (e.g. yank, visual, paste)"
                bind:value={cheatQuery}
              />
              {#if cheatQuery}
                <button class="btn icon" title="Clear" on:click={() => (cheatQuery = "")}>
                  <Icon icon="solar:close-circle-linear" size={12} />
                </button>
              {/if}
            </div>
          </div>
          {#if filteredCheatSheet.length === 0}
            <div class="empty">No bindings match "{cheatQuery}".</div>
          {:else}
            <div class="vim-cheat-grid">
              {#each filteredCheatSheet as group}
                <div class="vim-cheat-group">
                  <div class="vim-cheat-title">{group.title}</div>
                  <div class="vim-cheat-rows">
                    {#each group.rows as row}
                      <div class="vim-cheat-row">
                        <span class="key">{row.keys}</span>
                        <span class="vim-cheat-desc">{row.desc}</span>
                      </div>
                    {/each}
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>

      {:else if tab === "adapters"}
        <h2>Adapters</h2>
        <p class="desc">Debug adapters power language-specific debugging. Install the ones you need directly from here.</p>

        {#each adapters as a (a.language)}
          <div class="card adapter-card" id="adapters-list">
            <div class="card-row">
              <div class="card-info">
                <div class="card-title">
                  {#if a.installed}
                    <span class="status-dot ok" title="Installed"></span>
                  {:else if a.installing}
                    <span class="status-dot installing" title="Installing…"></span>
                  {:else}
                    <span class="status-dot missing" title="Not installed"></span>
                  {/if}
                  {a.label}
                </div>
                <span class="card-desc">{a.description}</span>
                {#if a.error}
                  <span class="card-desc" style="color:var(--danger)">{a.error}</span>
                {/if}
              </div>
              <div class="adapter-actions">
                {#if a.installing}
                  <span class="btn sm outlined" style="opacity:0.6">Installing…</span>
                {:else if a.installed}
                  <span class="status-badge ready">Ready</span>
                {:else}
                  <button class="btn outlined sm" on:click={() => installAdapter(a.language)}>
                    <Icon icon="solar:download-linear" size={11} /> Install
                  </button>
                {/if}
              </div>
            </div>
            {#if !a.installed && !a.installing}
              <div class="card-row">
                <code class="install-cmd">{a.installCmd}</code>
                {#if a.installUrl}
                  <a href={a.installUrl} target="_blank" rel="noopener" class="btn icon sm" title="Documentation">
                    <Icon icon="solar:link-square-linear" size={12} />
                  </a>
                {/if}
              </div>
            {/if}
            {#if (adapterOutput[a.language]?.length ?? 0) > 0}
              <div class="card-row install-output-row">
                <pre class="install-output">{adapterOutput[a.language]}</pre>
              </div>
            {/if}
          </div>
        {/each}

        {#if adapters.length === 0}
          <div class="empty">Loading adapters…</div>
        {/if}

      {:else if tab === "general"}
        <h2>General</h2>
        <div class="card" id="general-toggles">
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
        </div>

        <div class="card" id="general-shortcuts">
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

        <div class="card" id="general-updates">
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

        <div class="card" id="general-about">
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

        <div class="field" id="general-reset">
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

  .search-results {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .search-result {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    background: transparent;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
    text-align: left;
    padding: 10px 12px;
    cursor: pointer;
    transition: border-color 80ms ease, background 80ms ease;
  }
  .search-result:hover {
    border-color: var(--accent);
    background: var(--bg-subtle);
  }
  .sr-body {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .sr-title {
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sr-desc {
    color: var(--text-faint);
    font-size: 11px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sr-tab {
    color: var(--text-faint);
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.5px;
    text-transform: uppercase;
    padding: 2px 6px;
    background: var(--bg-subtle);
    border: 1px solid var(--border-subtle);
    border-radius: 3px;
    flex-shrink: 0;
  }
  .search-empty {
    padding: 24px 12px;
    text-align: center;
    color: var(--text-faint);
    font-size: 13px;
  }

  /* Pulse a card when search jumps to it. */
  :global(.flash-highlight) {
    animation: settings-flash 1.4s ease-out;
  }
  @keyframes settings-flash {
    0%   { box-shadow: 0 0 0 2px var(--accent), 0 0 0 6px rgba(94, 166, 255, 0.25); }
    100% { box-shadow: 0 0 0 0 transparent, 0 0 0 0 transparent; }
  }

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

  /* Vim mappings */
  .disabled-card { opacity: 0.55; }
  .vim-map-header { display:flex; align-items:flex-start; gap:var(--space-3); justify-content:space-between; }
  .vim-map-header .card-info { flex:1; min-width:0; }
  .vim-map-table { display:flex; flex-direction:column; }
  .vim-map-row {
    display:grid;
    grid-template-columns: 110px 1fr 1.4fr auto;
    gap:var(--space-2);
    align-items:center;
    padding:var(--space-1) var(--space-3);
    border-bottom:1px solid var(--border-subtle);
  }
  .vim-map-row:last-child { border-bottom:none; }
  .vim-map-head {
    font-size:var(--text-xs);
    color:var(--text-faint);
    text-transform:uppercase;
    letter-spacing:0.04em;
    padding-top:var(--space-2);
    padding-bottom:var(--space-2);
  }
  .vim-map-row select,
  .vim-map-row input[type="text"] {
    background:var(--bg);
    border:1px solid var(--border);
    border-radius:var(--radius-sm);
    color:var(--text);
    font-family:var(--font-mono);
    font-size:var(--text-sm);
    padding:4px 8px;
    width:100%;
    min-width:0;
  }
  .vim-map-row select:focus,
  .vim-map-row input[type="text"]:focus {
    outline:none;
    border-color:var(--accent);
  }

  /* Vim suggestion chips */
  .vim-suggest {
    display:flex;
    flex-direction:column;
    gap:6px;
    padding:var(--space-2) var(--space-3);
    border-bottom:1px solid var(--border-subtle);
    background:var(--bg-subtle);
  }
  .vim-suggest-label {
    font-size:var(--text-xs);
    text-transform:uppercase;
    letter-spacing:0.04em;
    color:var(--text-faint);
  }
  .vim-suggest-chips {
    display:flex;
    flex-wrap:wrap;
    gap:6px;
  }
  .vim-suggest-chip {
    display:inline-flex;
    align-items:center;
    gap:6px;
    padding:3px 8px;
    background:var(--bg);
    border:1px solid var(--border);
    border-radius:var(--radius-sm);
    font-size:var(--text-xs);
    color:var(--text-muted);
    cursor:pointer;
    transition:border-color 80ms ease, background 80ms ease;
  }
  .vim-suggest-chip:hover:not(:disabled) {
    border-color:var(--accent);
    color:var(--text);
  }
  .vim-suggest-chip:disabled { cursor:default; opacity:0.6; }
  .vim-suggest-chip.applied {
    border-color:var(--success);
    background:color-mix(in srgb, var(--success) 8%, transparent);
  }
  .vim-suggest-mode {
    font-family:var(--font-mono);
    font-size:9px;
    text-transform:uppercase;
    letter-spacing:0.04em;
    color:var(--text-faint);
    padding:1px 4px;
    background:var(--bg-subtle);
    border-radius:3px;
  }
  .vim-suggest-keys { display:inline-flex; align-items:center; gap:4px; }
  .vim-suggest-keys .key { margin-right:0; min-width:0; padding:1px 5px; }

  /* Vim cheat sheet */
  .vim-cheat-header {
    display:flex;
    align-items:center;
    justify-content:space-between;
    gap:var(--space-3);
  }
  .vim-cheat-search {
    display:inline-flex;
    align-items:center;
    gap:4px;
    background:var(--bg);
    border:1px solid var(--border);
    border-radius:var(--radius-sm);
    padding:2px 6px;
    min-width:240px;
  }
  .vim-cheat-search:focus-within { border-color:var(--accent); }
  .vim-cheat-search input {
    flex:1;
    min-width:0;
    background:transparent;
    border:none;
    outline:none;
    color:var(--text);
    font-size:var(--text-sm);
    font-family:var(--font-mono);
    padding:2px 0;
  }
  .vim-cheat-grid {
    display:grid;
    grid-template-columns:repeat(auto-fill, minmax(280px, 1fr));
    gap:var(--space-3);
    padding:var(--space-3);
  }
  .vim-cheat-group {
    border:1px solid var(--border-subtle);
    border-radius:var(--radius-sm);
    background:var(--bg);
  }
  .vim-cheat-title {
    font-size:var(--text-xs);
    text-transform:uppercase;
    letter-spacing:0.04em;
    color:var(--text-faint);
    padding:var(--space-2) var(--space-3);
    border-bottom:1px solid var(--border-subtle);
  }
  .vim-cheat-rows { display:flex; flex-direction:column; }
  .vim-cheat-row {
    display:grid;
    grid-template-columns:auto 1fr;
    align-items:center;
    gap:var(--space-2);
    padding:4px var(--space-3);
    font-size:var(--text-sm);
  }
  .vim-cheat-row .key {
    margin-right:0;
    min-width:0;
    padding:1px 8px;
    white-space:nowrap;
  }
  .vim-cheat-desc { color:var(--text-muted); }

  /* Adapters */
  .adapter-card .card-row { align-items: flex-start; }
  .adapter-actions { display:flex; align-items:center; gap:var(--space-2); flex-shrink:0; }
  .status-dot {
    display:inline-block; width:7px; height:7px; border-radius:50%;
    margin-right:4px; vertical-align:middle; flex-shrink:0;
  }
  .status-dot.ok { background:var(--success); }
  .status-dot.missing { background:var(--text-faint); }
  .status-dot.installing { background:var(--warning); animation:pulse-dot 1s ease-in-out infinite; }
  @keyframes pulse-dot { 0%,100% { opacity:1; } 50% { opacity:0.3; } }

  .status-badge {
    font-size:var(--text-xs); font-weight:600; padding:2px 8px;
    border-radius:var(--radius-sm); background:var(--bg-accent);
    color:var(--success); white-space:nowrap;
  }
  .status-badge.ready { background:var(--bg-accent); color:var(--success); }

  .install-cmd {
    font-size:var(--text-xs); padding:4px 8px; background:var(--bg);
    border-radius:var(--radius-sm); color:var(--text-muted);
    word-break:break-all;
  }
  .install-output-row { flex-direction:column; align-items:stretch; }
  .install-output {
    font-size:var(--text-xs); background:var(--bg); color:var(--text-muted);
    border-radius:var(--radius-sm); padding:var(--space-2); margin:0;
    max-height:200px; overflow-y:auto; white-space:pre-wrap; word-break:break-all;
    line-height:1.4;
  }

  .desc { color:var(--text-muted); font-size:var(--text-sm); margin-bottom:var(--space-3); }
</style>
