<script lang="ts">
  import { layout, setInspectorActive, toggleEnvDrawer, type InspectorId } from "./panels/layout";
  import VariablesPanel from "./VariablesPanel.svelte";
  import WatchPanel from "./WatchPanel.svelte";
  import CallStackPanel from "./CallStackPanel.svelte";
  import ThreadsPanel from "./ThreadsPanel.svelte";
  import ResourcesPanel from "./ResourcesPanel.svelte";
  import EnvPanel from "./EnvPanel.svelte";
  import Icon from "./Icon.svelte";

  const tabs: { id: InspectorId; label: string }[] = [
    { id: "variables", label: "Variables" },
    { id: "watch",     label: "Watch" },
    { id: "callstack", label: "Stack" },
    { id: "threads",   label: "Threads" },
    { id: "resources", label: "Resources" },
  ];

  $: active = $layout.inspectorActive;
  $: envExpanded = $layout.envExpanded;
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

  <!-- Env drawer: header always visible, body only when expanded. -->
  <div class="env-drawer" class:expanded={envExpanded}>
    <button
      class="env-head"
      class:expanded={envExpanded}
      aria-expanded={envExpanded}
      on:click={toggleEnvDrawer}
    >
      <span class="chev">
        <Icon icon="solar:alt-arrow-right-linear" size={10} />
      </span>
      <span class="label">Env</span>
    </button>
    {#if envExpanded}
      <div class="env-body">
        <EnvPanel />
      </div>
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

  /* Env drawer */
  .env-drawer {
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    border-top: 1px solid var(--border-subtle);
  }
  .env-drawer.expanded {
    max-height: 45%;
    min-height: 140px;
  }
  .env-head {
    display: flex;
    align-items: center;
    gap: 4px;
    width: 100%;
    height: 26px;
    padding: 0 10px 0 8px;
    background: var(--bg-subtle);
    border: 0;
    color: var(--text-faint);
    cursor: pointer;
    font-size: 10px;
    font-family: var(--font-ui);
    font-weight: 700;
    letter-spacing: 0.8px;
    text-transform: uppercase;
    user-select: none;
    flex-shrink: 0;
  }
  .env-head:hover { color: var(--text-muted); }
  .env-head .chev {
    display: inline-flex;
    transition: transform 120ms ease;
  }
  .env-head.expanded .chev { transform: rotate(90deg); }
  .env-head .label { flex: 1; text-align: left; }

  .env-body {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--bg);
  }
</style>
