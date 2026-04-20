<script lang="ts">
  import { layout, setInspectorActive, type InspectorId } from "./panels/layout";
  import VariablesPanel from "./VariablesPanel.svelte";
  import WatchPanel from "./WatchPanel.svelte";
  import CallStackPanel from "./CallStackPanel.svelte";
  import ThreadsPanel from "./ThreadsPanel.svelte";
  import ResourcesPanel from "./ResourcesPanel.svelte";

  const tabs: { id: InspectorId; label: string }[] = [
    { id: "variables", label: "Variables" },
    { id: "watch",     label: "Watch" },
    { id: "callstack", label: "Stack" },
    { id: "threads",   label: "Threads" },
    { id: "resources", label: "Resources" },
  ];

  $: active = $layout.inspectorActive;
</script>

<aside class="inspector">
  <div class="head">
    <div class="segmented" role="tablist">
      {#each tabs as t}
        <button
          class="seg"
          class:active={active === t.id}
          role="tab"
          aria-selected={active === t.id}
          on:click={() => setInspectorActive(t.id)}
        >
          {t.label}
        </button>
      {/each}
    </div>
  </div>
  <div class="body">
    <div class="pane" hidden={active !== "variables"}>
      <VariablesPanel hideHeader />
    </div>
    <div class="pane" hidden={active !== "watch"}>
      <WatchPanel hideHeader />
    </div>
    <div class="pane" hidden={active !== "callstack"}>
      <CallStackPanel hideHeader />
    </div>
    <div class="pane" hidden={active !== "threads"}>
      <ThreadsPanel hideHeader />
    </div>
    <div class="pane" hidden={active !== "resources"}>
      <ResourcesPanel hideHeader />
    </div>
  </div>
</aside>

<style>
  .inspector {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: var(--bg);
    border-left: 1px solid var(--border-subtle);
  }
  :global(body.mac) .inspector { border-left-color: rgba(0, 0, 0, 0.35); }

  .head {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 40px;
    padding: 0 8px;
    flex-shrink: 0;
    border-bottom: 1px solid var(--border-subtle);
  }
  :global(body.mac) .head { border-bottom-color: rgba(0, 0, 0, 0.3); }
  /* On 5 tabs make the segmented a bit tighter */
  .head .segmented .seg { padding: 0 8px; }

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
