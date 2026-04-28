<script lang="ts">
  import { onMount } from "svelte";
  import {
    activeSession,
    activeSessionId,
    sessions,
    workspace,
    control,
    stopSession,
    startSession,
    pickDebugFile,
    cleanDebugBinaries,
    manualSourcePath,
  } from "./store";
  import { toggleArea, setActivePanel } from "./panels/layout";
  import {
    themeList,
    currentThemeName,
    refreshThemeList,
    setTheme,
    loadTheme,
    type ThemeMeta,
  } from "./theme-engine";
  import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";
  import Icon from "./Icon.svelte";

  export let open = false;
  export let onOpenSettings: () => void = () => {};
  export let onOpenImport: () => void = () => {};
  export let onOpenFile: () => void = () => {};

  type Mode = "commands" | "themes" | "run";
  let mode: Mode = "commands";
  let input = "";
  let selected = 0;
  let savedTheme = "";

  // File search index — populated lazily on first palette open.
  let projectFiles: string[] = [];
  let projectFilesRoot = "";
  let projectFilesLoading = false;

  type ActionKind = "command" | "run" | "theme" | "file";
  type Action = {
    id: string;
    label: string;
    hint?: string;
    kind?: ActionKind;
    detail?: string;
    run: () => void;
  };

  $: currentList = mode === "commands" ? buildActions()
    : mode === "themes" ? buildThemeList()
    : buildRunList();

  $: filtered = currentList.filter((a) => fuzzy(input, a.label));

  // In commands mode, if user types something matching a run config, show run items too
  $: searchHitsRun = mode === "commands" && input.length >= 2
    ? buildRunList().filter((a) => fuzzy(input, a.label))
    : [];
  // In commands mode, search the project's Go files when the user starts typing.
  $: searchHitsFiles = mode === "commands" && input.length >= 2
    ? buildFileList()
        .filter((a) => fuzzy(input, (a.detail ? a.detail + "/" : "") + a.label))
        .slice(0, 12)
    : [];
  $: displayList = mode === "commands"
    ? [...filtered, ...searchHitsRun, ...searchHitsFiles]
    : filtered;

  // Live preview: when arrowing through themes, apply the selected one
  $: if (mode === "themes" && filtered.length > 0) {
    const t = filtered[Math.min(selected, filtered.length - 1)];
    if (t) loadTheme(t.id);
  }

  function buildActions(): Action[] {
    const base: Action[] = [
      {
        id: "settings.open",
        label: "Settings: Open Settings",
        hint: "⌘,",
        run: () => onOpenSettings(),
      },
      {
        id: "appearance.theme",
        label: "Appearance: Switch Theme…",
        hint: "⌘K ⌘T",
        run: () => enterThemeMode(),
      },
      {
        id: "debug.run",
        label: "Debug: Run…",
        hint: "→",
        run: () => enterRunMode(),
      },
      {
        id: "workspace.openJson",
        label: "Workspace: Open debug.json…",
        run: () => pickDebugFile(),
      },
      {
        id: "debugfiles.detect",
        label: "Debug Files: Auto-detect configs…",
        run: () => onOpenImport(),
      },
      {
        id: "debug.cleanBinaries",
        label: "Debug: Clean debug binaries",
        run: () => cleanDebugBinaries(),
      },
      { id: "view.sidebar",   label: "View: Toggle Sidebar",   hint: "⌘0",   run: () => toggleArea("sidebar") },
      { id: "view.inspector", label: "View: Toggle Inspector", hint: "⌘⌥0",  run: () => toggleArea("inspector") },
    ];
    if ($activeSessionId) {
      const id = $activeSessionId;
      base.push({ id: "debug.continue", label: "Debug: Continue", hint: "F5", run: () => control("Continue", id) });
      base.push({ id: "debug.pause", label: "Debug: Pause", run: () => control("Pause", id) });
      base.push({ id: "debug.stepOver", label: "Debug: Step Over", hint: "F10", run: () => control("StepOver", id) });
      base.push({ id: "debug.stepIn", label: "Debug: Step In", hint: "F11", run: () => control("StepIn", id) });
      base.push({ id: "debug.stepOut", label: "Debug: Step Out", hint: "⇧F11", run: () => control("StepOut", id) });
      base.push({ id: "debug.stop", label: "Debug: Stop", hint: "⇧F5", run: () => stopSession(id) });
    }
    return base;
  }

  function buildRunList(): Action[] {
    const runningSessions = Object.values($sessions);
    const runningCfgIds = new Set(runningSessions.map(s => s.cfgId));
    return ($workspace?.configs ?? [])
      .filter(cfg => !cfg.disabled && !runningCfgIds.has(cfg.id))
      .map((cfg) => ({
        id: "start." + cfg.id,
        label: cfg.label,
        hint: cfg.mode ?? "debug",
        kind: "run",
        run: () => startSession(cfg.id),
      }));
  }

  function buildFileList(): Action[] {
    if (!projectFilesRoot) return [];
    return projectFiles.map((rel) => {
      const slash = rel.lastIndexOf("/");
      const name = slash >= 0 ? rel.slice(slash + 1) : rel;
      const dir = slash >= 0 ? rel.slice(0, slash) : "";
      return {
        id: "file." + rel,
        label: name,
        detail: dir,
        kind: "file" as const,
        run: () => openFile(rel),
      };
    });
  }

  function openFile(rel: string) {
    if (!projectFilesRoot) return;
    manualSourcePath.set(projectFilesRoot + "/" + rel);
    setActivePanel("right", "source");
  }

  async function ensureFileIndex() {
    const root = $workspace?.root ?? "";
    if (!root) return;
    if (root === projectFilesRoot && projectFiles.length > 0) return;
    if (projectFilesLoading) return;
    projectFilesLoading = true;
    try {
      projectFiles = (await FileService.ListGoFiles(root)) as string[] ?? [];
      projectFilesRoot = root;
    } catch {
      projectFiles = [];
    } finally {
      projectFilesLoading = false;
    }
  }

  // Load the file index the first time the palette opens (and on workspace change).
  $: if (open) ensureFileIndex();
  $: if (($workspace?.root ?? "") !== projectFilesRoot) {
    projectFiles = [];
    projectFilesRoot = "";
  }

  function enterRunMode() {
    mode = "run";
    input = "";
    selected = 0;
  }

  function buildThemeList(): Action[] {
    return ($themeList ?? []).map((t) => ({
      id: t.name,
      label: t.name,
      hint: t.appearance,
      run: () => confirmTheme(t.name),
    }));
  }

  function enterThemeMode() {
    savedTheme = $currentThemeName;
    mode = "themes";
    input = "";
    selected = 0;
    refreshThemeList();
  }

  function confirmTheme(name: string) {
    setTheme(name);
    savedTheme = "";
    close();
  }

  function fuzzy(q: string, s: string): boolean {
    if (!q) return true;
    const hay = s.toLowerCase();
    const needle = q.toLowerCase();
    let i = 0;
    for (const c of hay) {
      if (c === needle[i]) i++;
      if (i === needle.length) return true;
    }
    return false;
  }

  function close() {
    if (mode === "themes" && savedTheme) {
      loadTheme(savedTheme);
      savedTheme = "";
    }
    open = false;
    mode = "commands";
    input = "";
    selected = 0;
  }

  function goBack() {
    if (mode === "themes") {
      if (savedTheme) loadTheme(savedTheme);
      savedTheme = "";
    }
    mode = "commands";
    input = "";
    selected = 0;
  }

  function runSelected() {
    const list = displayList;
    const a = list[selected];
    if (!a) return;
    // Submenu entries just switch mode
    if (a.id === "appearance.theme" || a.id === "debug.run") {
      a.run();
      return;
    }
    a.run();
    close();
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Escape") {
      if (mode !== "commands") goBack();
      else close();
    } else if (e.key === "Enter") {
      runSelected();
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      selected = Math.min(displayList.length - 1, selected + 1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      selected = Math.max(0, selected - 1);
    }
  }

  $: if (input !== undefined) selected = 0;

  let chordK = false;

  function onGlobal(e: KeyboardEvent) {
    const mod = e.metaKey || e.ctrlKey;

    if (mod && e.shiftKey && e.key.toLowerCase() === "p") {
      e.preventDefault();
      if (open) { close(); } else { open = true; }
      return;
    }
    if (mod && e.key === ",") {
      e.preventDefault();
      onOpenSettings();
      return;
    }
    // Cmd+O → quick open file
    if (mod && e.key.toLowerCase() === "o") {
      e.preventDefault();
      onOpenFile();
      return;
    }
    // Ctrl+` → focus terminal
    if (mod && e.key === "`") {
      e.preventDefault();
      import("./panels/layout").then(({ showBottomTab }) => {
        showBottomTab("terminal");
      });
      return;
    }
    if (mod && e.key.toLowerCase() === "k") {
      e.preventDefault();
      chordK = true;
      setTimeout(() => (chordK = false), 1000);
      return;
    }
    if (chordK && mod && e.key.toLowerCase() === "t") {
      e.preventDefault();
      chordK = false;
      open = true;
      enterThemeMode();
      return;
    }
    if (chordK) chordK = false;

    if (open) return;
    const id = $activeSessionId;
    if (!id) return;
    if (e.key === "F5") {
      e.preventDefault();
      if (e.shiftKey) stopSession(id);
      else control("Continue", id);
    } else if (e.key === "F10") {
      e.preventDefault();
      control("StepOver", id);
    } else if (e.key === "F11") {
      e.preventDefault();
      if (e.shiftKey) control("StepOut", id);
      else control("StepIn", id);
    }
  }

  onMount(() => {
    window.addEventListener("keydown", onGlobal);
    return () => window.removeEventListener("keydown", onGlobal);
  });

  $: if (!open) {
    mode = "commands";
    input = "";
    selected = 0;
  }
</script>

{#if open}
  <div
    class="backdrop"
    role="presentation"
    on:click={close}
    on:keydown={(e) => e.key === "Escape" && close()}
  ></div>
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div class="palette" role="dialog" aria-modal="true" on:click|stopPropagation on:keydown|stopPropagation>
    <div class="search-row">
      {#if mode !== "commands"}
        <button class="back btn icon" on:click={goBack} title="Back (Esc)">
          <Icon icon="solar:arrow-left-linear" size={14} />
        </button>
      {/if}
      <!-- svelte-ignore a11y-autofocus -->
      <input
        class="tx search"
        placeholder={mode === "themes" ? "Select a theme…" : mode === "run" ? "Select a debug config…" : "Type a command…"}
        bind:value={input}
        on:keydown={onKey}
        autofocus
      />
    </div>
    {#if mode !== "commands"}
      <div class="mode-label">
        {mode === "themes" ? "Themes" : "Debug Configurations"}
      </div>
    {/if}
    <div class="list">
      {#each displayList as a, i}
        <button
          class="item"
          class:sel={i === selected}
          on:click={() => { selected = i; runSelected(); }}
          on:mouseenter={() => (selected = i)}
        >
          {#if mode === "themes"}
            <span class="theme-dot" class:active={a.id === $currentThemeName}>●</span>
          {:else if mode === "run" || a.kind === "run"}
            <Icon icon="solar:play-bold" size={12} color="var(--success)" />
          {:else if a.kind === "file"}
            <Icon icon="solar:code-bold" size={12} color="var(--info)" />
          {/if}
          <span class="label">{a.label}</span>
          {#if a.detail}
            <span class="detail">{a.detail}</span>
          {/if}
          {#if a.hint}
            <span class="hint">{a.hint}</span>
          {/if}
        </button>
      {/each}
      {#if displayList.length === 0}
        <div class="empty">No matches</div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.35);
    z-index: 900;
  }
  .palette {
    position: fixed;
    top: 20vh;
    left: 50%;
    transform: translateX(-50%);
    width: 520px;
    max-height: 60vh;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.5);
    z-index: 901;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .search-row {
    display: flex;
    align-items: center;
    border-bottom: 1px solid var(--border);
  }
  .back {
    margin-left: var(--space-2);
  }
  .search {
    flex: 1;
    height: 40px;
    border: 0;
    border-radius: 0;
    background: transparent;
    padding: 0 var(--space-4);
    font-size: var(--text-md);
  }
  .search:focus {
    box-shadow: none;
  }
  .list {
    overflow: auto;
    padding: var(--space-1) 0;
  }
  .item {
    display: flex;
    width: 100%;
    align-items: center;
    gap: var(--space-2);
    background: transparent;
    border: 0;
    padding: 7px var(--space-4);
    font-size: var(--text-sm);
    color: var(--text);
    cursor: pointer;
    text-align: left;
  }
  .item.sel {
    background: var(--accent-subtle);
  }
  .label {
    flex: 1;
  }
  .hint {
    color: var(--text-faint);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }
  .detail {
    color: var(--text-faint);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 240px;
  }
  .theme-dot {
    color: var(--text-faint);
    font-size: 10px;
  }
  .theme-dot.active {
    color: var(--accent);
  }
  .mode-label {
    padding: 4px var(--space-4);
    font-size: var(--text-xs);
    font-weight: 600;
    color: var(--text-faint);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .empty {
    padding: var(--space-3);
    color: var(--text-faint);
    font-size: var(--text-sm);
    text-align: center;
  }
</style>
