<script lang="ts">
  import { onMount } from "svelte";
  import { Splitpanes, Pane } from "svelte-splitpanes";
  import {
    workspace,
    sessions,
    sessionState,
    activeSessionId,
    activeSession,
    refreshWorkspace,
    refreshSessions,
    pickDebugFile,
    startSession,
    stopSession,
    control,
  } from "./lib/store";
  import Dock from "./lib/Dock.svelte";
  import StatusBar from "./lib/StatusBar.svelte";
  import CommandPalette from "./lib/CommandPalette.svelte";
  import SettingsPage from "./lib/SettingsPage.svelte";
  import Toast from "./lib/Toast.svelte";
  import ImportWizard from "./lib/ImportWizard.svelte";
  import Icon from "./lib/Icon.svelte";
  import { layout, setDockSize } from "./lib/panels/layout";

  let cfgPickerOpen = false;
  let paletteOpen = false;
  let settingsOpen = false;
  let importWizardOpen = false;

  onMount(async () => {
    await refreshWorkspace();
    await refreshSessions();
    const list = Object.values($sessions);
    if (list.length && !$activeSessionId) activeSessionId.set(list[0].id);
  });

  $: sessionList = Object.values($sessions);

  async function startFromPicker(cfgId: string) {
    cfgPickerOpen = false;
    await startSession(cfgId);
  }

  function onResize(e: CustomEvent<any>) {
    const panes = e.detail;
    if (panes?.length === 2) {
      setDockSize("left", panes[0].size);
      setDockSize("right", panes[1].size);
    }
  }
</script>

<CommandPalette
  bind:open={paletteOpen}
  onOpenSettings={() => (settingsOpen = true)}
  onOpenImport={() => (importWizardOpen = true)}
/>
<SettingsPage bind:open={settingsOpen} onOpenImport={() => (importWizardOpen = true)} />
<ImportWizard bind:open={importWizardOpen} />
<Toast />

<main>
  <!-- unified titlebar: drag region + tabs + controls -->
  <div class="titlebar" data-wml-drag>
    <!-- left: traffic light space + actions -->
    <div class="tb-left" data-wml-no-drag>
      <span class="logo">DelveUI</span>
      <button class="btn outlined tb-btn" on:click={() => pickDebugFile()}>
        <Icon icon="solar:document-bold" size={12} /> debug.json
      </button>
      <button class="btn icon tb-btn" title="Settings (⌘,)" on:click={() => (settingsOpen = true)}>
        <Icon icon="solar:settings-bold" size={13} />
      </button>
    </div>

    <!-- center: session tabs -->
    <div class="tb-center" data-wml-no-drag>
      {#each sessionList as s (s.id)}
        <button
          class="session-tab"
          class:active={s.id === $activeSessionId}
          on:click={() => activeSessionId.set(s.id)}
        >
          <span class="state-dot state-{s.state}">●</span>
          <span class="tab-label">{s.label}</span>
          <button
            class="tab-close"
            title="Stop & close"
            on:click|stopPropagation={() => stopSession(s.id)}
          >
            <Icon icon="solar:close-circle-linear" size={11} />
          </button>
        </button>
      {/each}
      {#if sessionList.length === 0}
        <span class="no-sessions">No sessions</span>
      {/if}
    </div>

    <!-- right: run + controls -->
    <div class="tb-right" data-wml-no-drag>
      <div class="picker">
        <button class="btn primary tb-btn" on:click={() => (cfgPickerOpen = !cfgPickerOpen)}>
          <Icon icon="solar:play-bold" size={12} />
          Run
          <Icon icon="solar:alt-arrow-down-linear" size={10} />
        </button>
        {#if cfgPickerOpen}
          <div class="dd">
            {#if !$workspace?.configs?.length}
              <div class="dd-empty">Open a debug.json first</div>
            {/if}
            {#each ($workspace?.configs ?? []).filter(c => !sessionList.some(s => s.cfgId === c.id)) as cfg}
              <button class="dd-item" on:click={() => startFromPicker(cfg.id)}>
                <Icon icon="solar:play-bold" size={12} color="var(--success)" />
                <span>{cfg.label}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>

      {#if $activeSession}
        <div class="controls">
          <button class="btn icon tb-btn" title="Continue (F5)" on:click={() => $activeSessionId && control("Continue", $activeSessionId)}>
            <Icon icon="solar:play-bold" size={13} />
          </button>
          <button class="btn icon tb-btn" title="Pause" on:click={() => $activeSessionId && control("Pause", $activeSessionId)}>
            <Icon icon="solar:pause-bold" size={13} />
          </button>
          <button class="btn icon tb-btn" title="Step Over" on:click={() => $activeSessionId && control("StepOver", $activeSessionId)}>
            <Icon icon="solar:forward-2-bold" size={13} />
          </button>
          <button class="btn icon tb-btn" title="Step In" on:click={() => $activeSessionId && control("StepIn", $activeSessionId)}>
            <Icon icon="solar:arrow-down-bold" size={13} />
          </button>
          <button class="btn icon tb-btn" title="Step Out" on:click={() => $activeSessionId && control("StepOut", $activeSessionId)}>
            <Icon icon="solar:arrow-up-bold" size={13} />
          </button>
          <button class="btn icon tb-btn danger" title="Stop" on:click={() => $activeSessionId && stopSession($activeSessionId)}>
            <Icon icon="solar:stop-bold" size={13} />
          </button>
        </div>
      {/if}
    </div>
  </div>

  <!-- two-panel layout -->
  <div class="workspace">
    <Splitpanes on:resized={onResize}>
      <Pane size={$layout.sizes.left} minSize={15}>
        <Dock dock="left" />
      </Pane>
      <Pane size={$layout.sizes.right} minSize={20}>
        <Dock dock="right" />
      </Pane>
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

  .titlebar {
    display: flex;
    align-items: center;
    height: 38px;
    background: var(--bg-elevated);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
    -webkit-app-region: drag;
    padding: 0 var(--space-2);
    gap: 0;
  }

  .tb-left, .tb-center, .tb-right {
    display: flex;
    align-items: center;
    -webkit-app-region: no-drag;
  }
  .tb-left {
    gap: var(--space-1);
    padding-left: 72px; /* macOS traffic light space */
  }
  .tb-center {
    flex: 1;
    justify-content: center;
    gap: 2px;
    overflow: hidden;
    padding: 0 var(--space-2);
  }
  .tb-right {
    gap: var(--space-1);
  }

  .tb-btn {
    height: 24px;
    padding: 0 8px;
    font-size: var(--text-xs);
  }

  .logo {
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.5px;
    color: var(--text-faint);
    margin-right: var(--space-1);
  }

  /* session tabs */
  .session-tab {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    height: 26px;
    padding: 0 var(--space-2) 0 var(--space-2);
    background: transparent;
    border: 0;
    border-bottom: 2px solid transparent;
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-family: var(--font-ui);
    cursor: pointer;
    white-space: nowrap;
    transition: color 80ms, border-color 80ms;
    position: relative;
  }
  .session-tab:hover {
    color: var(--text);
    background: var(--bg-subtle);
    border-radius: var(--radius-sm) var(--radius-sm) 0 0;
  }
  .session-tab.active {
    color: var(--text);
    border-bottom-color: var(--accent);
    background: var(--bg);
    border-radius: var(--radius-sm) var(--radius-sm) 0 0;
  }
  .tab-label {
    max-width: 140px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .tab-close {
    display: flex;
    align-items: center;
    background: transparent;
    border: 0;
    color: var(--text-faint);
    cursor: pointer;
    padding: 0;
    margin-left: 2px;
    opacity: 0;
    transition: opacity 80ms;
  }
  .session-tab:hover .tab-close,
  .session-tab.active .tab-close {
    opacity: 1;
  }
  .tab-close:hover {
    color: var(--danger);
  }

  .no-sessions {
    color: var(--text-faint);
    font-size: var(--text-xs);
  }

  /* dropdowns */
  .picker { position: relative; }
  .dd {
    position: absolute;
    top: calc(100% + 4px);
    right: 0;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    min-width: 260px;
    z-index: 100;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
    padding: var(--space-1) 0;
    max-height: 320px;
    overflow: auto;
  }
  .dd-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    background: transparent;
    border: 0;
    padding: 6px var(--space-3);
    color: var(--text);
    font-size: var(--text-sm);
    text-align: left;
    cursor: pointer;
  }
  .dd-item:hover { background: var(--bg-subtle); }
  .dd-empty { padding: var(--space-2) var(--space-3); color: var(--text-faint); font-size: var(--text-sm); }

  .controls { display: flex; gap: 1px; }

  .workspace { flex: 1; min-height: 0; overflow: hidden; }
</style>
