<script lang="ts">
  import { onMount } from "svelte";
  import { Splitpanes, Pane } from "svelte-splitpanes";
  import {
    workspace,
    sessions,
    activeSessionId,
    activeSession,
    refreshWorkspace,
    refreshSessions,
    pickDebugFile,
    stopSession,
    restartSession,
    control,
  } from "./lib/store";
  import Sidebar from "./lib/Sidebar.svelte";
  import CenterPanel from "./lib/CenterPanel.svelte";
  import Inspector from "./lib/Inspector.svelte";
  import StatusBar from "./lib/StatusBar.svelte";
  import CommandPalette from "./lib/CommandPalette.svelte";
  import SettingsPage from "./lib/SettingsPage.svelte";
  // ImportWizard removed: editor auto-detection retired in favor of explicit
  // file/folder picks (see ProjectSwitcher / WelcomePage).
  import Toast from "./lib/Toast.svelte";
  import WelcomePage from "./lib/WelcomePage.svelte";
  import ConfigPicker from "./lib/ConfigPicker.svelte";
  import QuickOpen from "./lib/QuickOpen.svelte";
  import Icon from "./lib/Icon.svelte";
  import ProjectSwitcher from "./lib/ProjectSwitcher.svelte";
  import RunPalette from "./lib/RunPalette.svelte";
  import { layout, setAreaSize, toggleArea, setSidebarActive, setAreaVisible } from "./lib/panels/layout";
  import { startMainThreadProbe } from "./lib/diagnostics";
  import { initSearchEvents } from "./lib/search-store";

  let runPaletteOpen = false;
  let paletteOpen = false;
  let settingsOpen = false;
  let configPickerOpen = false;
  let quickOpenOpen = false;
  let showWelcome = false;

  onMount(async () => {
    startMainThreadProbe();
    if (/Mac/i.test(navigator.platform) || /Mac/i.test(navigator.userAgent)) {
      document.body.classList.add("mac");
    }
    const { Events } = await import("@wailsio/runtime");
    Events.On("menu:command-palette", () => (paletteOpen = true));
    Events.On("menu:quick-open", () => (quickOpenOpen = true));
    Events.On("menu:open-debug-file", () => pickDebugFile());
    Events.On("menu:debug-control", (e: any) => {
      const action = (e?.data ?? e) as string;
      const id = $activeSessionId;
      if (!id) return;
      if (action === "Stop") stopSession(id);
      else control(action as any, id);
    });

    window.addEventListener("keydown", onLayoutKey);
    initSearchEvents();
    await refreshWorkspace();
    await refreshSessions();
    const list = Object.values($sessions);
    if (list.length && !$activeSessionId) activeSessionId.set(list[0].id);

    const { loadDebugFiles } = await import("./lib/settings-store");
    await loadDebugFiles();
    // Welcome page shows when nothing is open — the Welcome page itself lists
    // recents alongside Open Folder, so it's a useful landing surface even
    // when the user already has registered projects (e.g. they turned off
    // "Restore last project" in settings).
    if (!$workspace?.root && !$workspace?.debugFile) {
      showWelcome = true;
    }

    return () => window.removeEventListener("keydown", onLayoutKey);
  });

  function onLayoutKey(e: KeyboardEvent) {
    const mod = e.metaKey || e.ctrlKey;
    if (!mod) return;
    // Cmd+0 → toggle sidebar
    if (!e.altKey && !e.shiftKey && e.key === "0") {
      e.preventDefault();
      toggleArea("sidebar");
      return;
    }
    // Cmd+Alt+0 → toggle inspector
    if (e.altKey && !e.shiftKey && e.key === "0") {
      e.preventDefault();
      toggleArea("inspector");
      return;
    }
    // Cmd+Shift+F → reveal Search sidebar tab and focus the query input.
    if (e.shiftKey && !e.altKey && e.key.toLowerCase() === "f") {
      e.preventDefault();
      setAreaVisible("sidebar", true);
      setSidebarActive("search");
      return;
    }
  }


  async function onToolbarDblClick(e: MouseEvent) {
    // Only zoom if the click landed on the drag region itself, not on an
    // interactive descendant (buttons, inputs mark themselves no-drag).
    const el = e.target as HTMLElement;
    const style = getComputedStyle(el);
    if (style.getPropertyValue("--wails-draggable").trim() !== "drag") return;
    try {
      const { Window } = await import("@wailsio/runtime");
      await Window.ToggleMaximise();
    } catch (err) {
      console.error("zoom failed:", err);
    }
  }

  function onResize(e: CustomEvent<any>) {
    const panes = e.detail;
    if (!panes) return;
    const vis = $layout.visible;
    if (vis.sidebar && vis.inspector && panes.length === 3) {
      setAreaSize("sidebar", panes[0].size);
      setAreaSize("inspector", panes[2].size);
    } else if (vis.sidebar && !vis.inspector && panes.length === 2) {
      setAreaSize("sidebar", panes[0].size);
    } else if (!vis.sidebar && vis.inspector && panes.length === 2) {
      setAreaSize("inspector", panes[1].size);
    }
  }
</script>

<CommandPalette
  bind:open={paletteOpen}
  onOpenSettings={() => (settingsOpen = true)}
  onOpenFile={() => (quickOpenOpen = true)}
/>
<SettingsPage bind:open={settingsOpen} />
<ConfigPicker bind:open={configPickerOpen} />
<QuickOpen bind:open={quickOpenOpen} />
<RunPalette bind:open={runPaletteOpen} />

<WelcomePage visible={showWelcome} onDone={() => { showWelcome = false; refreshWorkspace(); }} />
<Toast />

<main>
  <!-- Unified toolbar -->
  <div class="toolbar" on:dblclick={onToolbarDblClick}>
    <div class="tb-trafficlights"></div>

    <div class="tb-left">
      <ProjectSwitcher />
    </div>

    <div class="tb-center">
      <button class="tb-cmd" title="Command Palette (⌘⇧P)" on:click={() => (paletteOpen = true)}>
        <Icon icon="solar:command-linear" size={11} />
        <span>Search anything…</span>
        <span class="kbd">⌘⇧P</span>
      </button>
    </div>

    <div class="tb-right">
      {#if $activeSession}
        <div class="segmented step-controls">
          <button class="seg" title="Continue (F5)" on:click={() => $activeSessionId && control("Continue", $activeSessionId)}>
            <Icon icon="solar:play-bold" size={12} />
          </button>
          <button class="seg" title="Pause" on:click={() => $activeSessionId && control("Pause", $activeSessionId)}>
            <Icon icon="solar:pause-bold" size={12} />
          </button>
          <button class="seg" title="Step Over (F10)" on:click={() => $activeSessionId && control("StepOver", $activeSessionId)}>
            <Icon icon="solar:forward-2-bold" size={12} />
          </button>
          <button class="seg" title="Step In (F11)" on:click={() => $activeSessionId && control("StepIn", $activeSessionId)}>
            <Icon icon="solar:arrow-down-bold" size={12} />
          </button>
          <button class="seg" title="Step Out (⇧F11)" on:click={() => $activeSessionId && control("StepOut", $activeSessionId)}>
            <Icon icon="solar:arrow-up-bold" size={12} />
          </button>
          <button class="seg" title="Restart" on:click={() => $activeSessionId && restartSession($activeSessionId)}>
            <Icon icon="solar:restart-bold" size={12} />
          </button>
        </div>
        <button class="tb-pill danger" title="Stop (⇧F5)" on:click={() => $activeSessionId && stopSession($activeSessionId)}>
          <Icon icon="solar:stop-bold" size={13} />
        </button>
      {/if}

      <button class="tb-icon" title="Settings (⌘,)" on:click={() => (settingsOpen = true)}>
        <Icon icon="solar:settings-linear" size={14} />
      </button>

      <button class="tb-pill primary" on:click={() => (runPaletteOpen = true)}>
        <Icon icon="solar:play-bold" size={12} />
        Run
      </button>
    </div>
  </div>

  <!-- 3-column horizontal layout. Center always fills full height. -->
  <div class="workspace">
    <Splitpanes on:resized={onResize}>
      {#if $layout.visible.sidebar}
        <Pane size={$layout.sizes.sidebar} minSize={12} maxSize={35}>
          <Sidebar />
        </Pane>
      {/if}

      <Pane minSize={30}>
        <CenterPanel />
      </Pane>

      {#if $layout.visible.inspector}
        <Pane size={$layout.sizes.inspector} minSize={14} maxSize={40}>
          <Inspector />
        </Pane>
      {/if}
    </Splitpanes>
  </div>

  <StatusBar />
</main>

<style>
  main {
    display: flex;
    flex-direction: column;
    height: 100vh;
    overflow: hidden;
  }

  /* ---- Unified toolbar ---- */
  .toolbar {
    display: flex;
    align-items: center;
    height: 48px;
    background: var(--bg-elevated);
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
    padding: 0 12px 0 0;
    gap: 0;
    /* Wails v3 window drag: whole bar is a drag handle. Interactive
       children opt out with --wails-draggable: no-drag. */
    --wails-draggable: drag;
  }
  :global(body.mac) .toolbar {
    height: 52px;
    background: linear-gradient(to bottom, var(--bg-elevated), var(--bg));
    border-bottom-color: var(--border-subtle);
    box-shadow: inset 0 -1px 0 rgba(255, 255, 255, 0.03);
  }

  /* Reserve space for the native traffic lights (drag region inherits). */
  .tb-trafficlights {
    width: 78px;
    height: 100%;
    flex-shrink: 0;
  }
  /* Interactive zone on the left — sits next to traffic lights. */
  .tb-left {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-left: 4px;
    --wails-draggable: no-drag;
  }

  /* Centered command-palette button. The wrapping zone stays draggable;
     only the inner button opts out so the rest of the empty bar still drags. */
  .tb-center {
    flex: 1;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    min-width: 0;
    padding: 0 12px;
  }
  .tb-cmd {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 4px 10px 4px 10px;
    background: var(--bg);
    border: 1px solid var(--border-subtle);
    border-radius: 20px;
    color: var(--text-muted);
    font: inherit;
    font-size: var(--text-sm);
    cursor: pointer;
    width: min(360px, 50%);
    --wails-draggable: no-drag;
  }
  .tb-cmd:hover { background: var(--bg-elevated); border-color: var(--border); color: var(--text); }
  .tb-cmd > span:not(.kbd) {
    flex: 1;
    text-align: left;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .kbd {
    font-family: var(--font-mono);
    font-size: 10px;
    padding: 1px 5px;
    border: 1px solid var(--border-subtle);
    border-radius: 4px;
    color: var(--text-faint);
    background: var(--bg-elevated);
  }

  /* Interactive zone on the right — buttons must not drag the window. */
  .tb-right {
    display: flex;
    align-items: center;
    gap: 8px;
    --wails-draggable: no-drag;
  }
  .tb-icon {
    width: 28px;
    height: 26px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 6px;
    color: var(--text-muted);
    cursor: pointer;
  }
  .tb-icon:hover { background: rgba(255, 255, 255, 0.06); color: var(--text); border-color: var(--border-subtle); }
  /* The segmented step-control group is also interactive. */
  .step-controls { --wails-draggable: no-drag; }
  .step-controls .seg { width: 28px; padding: 0; }

  .workspace {
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }
</style>
