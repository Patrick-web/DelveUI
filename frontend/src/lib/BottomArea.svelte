<script lang="ts">
  import { layout, setBottomActive, setAreaVisible, type BottomId } from "./panels/layout";
  import TerminalPanel from "./TerminalPanel.svelte";
  import ConsolePanel from "./ConsolePanel.svelte";
  import Icon from "./Icon.svelte";

  const tabs: { id: BottomId; label: string }[] = [
    { id: "terminal", label: "Terminal" },
    { id: "console",  label: "Debug Console" },
  ];

  $: active = $layout.bottomActive;
</script>

<div class="bottom">
  <div class="tabs">
    {#each tabs as t}
      <button
        class="bt-tab"
        class:active={active === t.id}
        on:click={() => setBottomActive(t.id)}
      >
        {t.label}
      </button>
    {/each}
    <span class="spacer"></span>
    <button class="bt-close" title="Hide (⌘⇧Y)" on:click={() => setAreaVisible("bottom", false)}>
      <Icon icon="solar:close-circle-linear" size={14} />
    </button>
  </div>
  <div class="body" class:visible-terminal={active === "terminal"} class:visible-console={active === "console"}>
    <div class="pane" hidden={active !== "terminal"}>
      <TerminalPanel />
    </div>
    <div class="pane" hidden={active !== "console"}>
      <ConsolePanel />
    </div>
  </div>
</div>

<style>
  .bottom {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: var(--bg);
    border-top: 1px solid var(--border-subtle);
  }
  :global(body.mac) .bottom { border-top-color: rgba(0, 0, 0, 0.35); }

  .tabs {
    display: flex;
    align-items: center;
    height: 28px;
    padding: 0 4px 0 8px;
    flex-shrink: 0;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-subtle);
  }
  .bt-tab {
    background: transparent;
    border: 0;
    border-bottom: 2px solid transparent;
    color: var(--text-muted);
    padding: 0 10px;
    height: 28px;
    font-size: var(--text-xs);
    font-family: var(--font-ui);
    cursor: pointer;
    margin-bottom: -1px;
  }
  .bt-tab:hover { color: var(--text); }
  .bt-tab.active {
    color: var(--text);
    border-bottom-color: var(--accent);
  }
  .spacer { flex: 1; }
  .bt-close {
    background: transparent;
    border: 0;
    color: var(--text-faint);
    cursor: pointer;
    padding: 4px 6px;
    border-radius: 4px;
  }
  .bt-close:hover { color: var(--text); background: rgba(255, 255, 255, 0.06); }

  .body {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    position: relative;
  }
  .pane {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }
  .pane[hidden] { display: none; }
</style>
