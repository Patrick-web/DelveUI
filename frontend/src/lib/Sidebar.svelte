<script lang="ts">
  import { layout, setSidebarActive, type SidebarTabId } from "./panels/layout";
  import SessionsPanel from "./SessionsPanel.svelte";
  import FileTreePanel from "./FileTreePanel.svelte";
  import BreakpointsPanel from "./BreakpointsPanel.svelte";

  const tabs: { id: SidebarTabId; label: string }[] = [
    { id: "sessions",    label: "Sessions" },
    { id: "filetree",    label: "Files" },
    { id: "breakpoints", label: "Breaks" },
  ];

  $: active = $layout.sidebarActive;
</script>

<aside class="sidebar">
  <div class="head">
    <div class="segmented" role="tablist">
      {#each tabs as t}
        <button
          class="seg"
          class:active={active === t.id}
          role="tab"
          aria-selected={active === t.id}
          on:click={() => setSidebarActive(t.id)}
        >
          {t.label}
        </button>
      {/each}
    </div>
  </div>
  <div class="body">
    <div class="pane" hidden={active !== "sessions"}>
      <SessionsPanel />
    </div>
    <div class="pane" hidden={active !== "filetree"}>
      <FileTreePanel hideHeader />
    </div>
    <div class="pane" hidden={active !== "breakpoints"}>
      <BreakpointsPanel hideHeader />
    </div>
  </div>
</aside>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    background: var(--bg-subtle);
    border-right: 1px solid var(--border-subtle);
  }
  :global(body.mac) .sidebar {
    border-right-color: rgba(0, 0, 0, 0.35);
    box-shadow: inset -1px 0 0 rgba(255, 255, 255, 0.025);
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
