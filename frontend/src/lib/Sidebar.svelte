<script lang="ts">
  import { layout, setSidebarActive, toggleBreakpointsDrawer, setBreakpointsExpanded, type SidebarTabId } from "./panels/layout";
  import { globalBreakpoints, activeSessionId, setBreakpoints } from "./store";
  import SessionsPanel from "./SessionsPanel.svelte";
  import FileTreePanel from "./FileTreePanel.svelte";
  import BreakpointsPanel from "./BreakpointsPanel.svelte";
  import Icon from "./Icon.svelte";

  const tabs: { id: SidebarTabId; label: string }[] = [
    { id: "sessions", label: "Sessions" },
    { id: "filetree", label: "Files" },
  ];

  $: active = $layout.sidebarActive;
  $: bpExpanded = $layout.breakpointsExpanded;

  $: bpCount = Object.values($globalBreakpoints).reduce(
    (n, lines) => n + (lines?.length ?? 0),
    0,
  );

  // Auto-expand the drawer the moment the first breakpoint appears.
  // Tracks transitions of bpCount: 0 → >0 expands; we don't auto-collapse.
  let prevBpCount = bpCount;
  $: {
    if (prevBpCount === 0 && bpCount > 0 && !bpExpanded) {
      setBreakpointsExpanded(true);
    }
    prevBpCount = bpCount;
  }

  async function clearAllBreakpoints(e: MouseEvent) {
    e.stopPropagation();
    const paths = Object.keys($globalBreakpoints);
    for (const p of paths) {
      await setBreakpoints($activeSessionId, p, []);
    }
  }
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
  </div>

  <!-- Breakpoints drawer: header always visible, body only when expanded. -->
  <div class="bp-drawer" class:expanded={bpExpanded}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div
      class="bp-head"
      class:expanded={bpExpanded}
      aria-expanded={bpExpanded}
      role="button"
      tabindex="0"
      on:click={toggleBreakpointsDrawer}
      on:keydown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); toggleBreakpointsDrawer(); } }}
    >
      <span class="chev">
        <Icon icon="solar:alt-arrow-right-linear" size={10} />
      </span>
      <span class="label">Breakpoints</span>
      {#if bpCount > 0}
        <span class="count">{bpCount}</span>
        <button
          class="clear"
          title="Clear all breakpoints"
          on:click={clearAllBreakpoints}
        >
          <Icon icon="solar:trash-bin-minimalistic-linear" size={11} />
        </button>
      {/if}
    </div>
    {#if bpExpanded}
      <div class="bp-body">
        <BreakpointsPanel hideHeader />
      </div>
    {/if}
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

  /* Breakpoints drawer — same pattern as the Env drawer in Inspector. */
  .bp-drawer {
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    border-top: 1px solid var(--border-subtle);
  }
  .bp-drawer.expanded {
    max-height: 45%;
    min-height: 140px;
  }
  .bp-head {
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
  .bp-head:hover { color: var(--text-muted); }
  .bp-head .chev {
    display: inline-flex;
    transition: transform 120ms ease;
  }
  .bp-head.expanded .chev { transform: rotate(90deg); }
  .bp-head .label { flex: 1; text-align: left; }
  .bp-head .count {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 16px;
    height: 14px;
    padding: 0 5px;
    border-radius: 7px;
    background: var(--danger);
    color: #fff;
    font-size: 9px;
    font-weight: 700;
    letter-spacing: 0;
    text-transform: none;
  }
  .bp-head .clear {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 18px;
    height: 18px;
    margin-left: 2px;
    background: transparent;
    border: 0;
    color: var(--text-faint);
    border-radius: 3px;
    cursor: pointer;
  }
  .bp-head .clear:hover {
    color: var(--danger);
    background: rgba(255,255,255,0.06);
  }

  .bp-body {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--bg);
  }
</style>
