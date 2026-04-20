<script lang="ts">
  import { layout, setInspectorActive, type InspectorId } from "./panels/layout";
  import VariablesPanel from "./VariablesPanel.svelte";
  import WatchPanel from "./WatchPanel.svelte";
  import ResourcesPanel from "./ResourcesPanel.svelte";

  const tabs: { id: InspectorId; label: string }[] = [
    { id: "variables", label: "Variables" },
    { id: "watch",     label: "Watch" },
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
    {#if active === "variables"}
      <VariablesPanel hideHeader />
    {:else if active === "watch"}
      <WatchPanel hideHeader />
    {:else if active === "resources"}
      <ResourcesPanel hideHeader />
    {/if}
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
  :global(body.mac) .inspector {
    border-left-color: rgba(0, 0, 0, 0.35);
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
  .body {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .body :global(> *) {
    flex: 1;
    min-height: 0;
  }
</style>
