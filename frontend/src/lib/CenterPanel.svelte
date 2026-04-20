<script lang="ts">
  import { layout, setCenterActive, type CenterTabId } from "./panels/layout";
  import SourcePanel from "./SourcePanel.svelte";
  import TerminalPanel from "./TerminalPanel.svelte";
  import ConsolePanel from "./ConsolePanel.svelte";

  const tabs: { id: CenterTabId; label: string }[] = [
    { id: "terminal", label: "Terminal" },
    { id: "console",  label: "Debug Console" },
    { id: "source",   label: "Source" },
  ];

  $: active = $layout.centerActive;
</script>

<div class="center">
  <div class="head">
    <div class="segmented" role="tablist">
      {#each tabs as t}
        <button
          class="seg"
          class:active={active === t.id}
          role="tab"
          aria-selected={active === t.id}
          on:click={() => setCenterActive(t.id)}
        >
          {t.label}
        </button>
      {/each}
    </div>
  </div>
  <div class="body">
    <div class="pane" hidden={active !== "terminal"}>
      <TerminalPanel />
    </div>
    <div class="pane" hidden={active !== "console"}>
      <ConsolePanel />
    </div>
    <div class="pane" hidden={active !== "source"}>
      <SourcePanel />
    </div>
  </div>
</div>

<style>
  .center {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: var(--bg);
    overflow: hidden;
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 40px;
    padding: 0 10px;
    flex-shrink: 0;
    border-bottom: 1px solid var(--border-subtle);
  }
  :global(body.mac) .head { border-bottom-color: rgba(0, 0, 0, 0.3); }

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
