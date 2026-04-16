<script lang="ts">
  import { layout, panelsInDock, setActivePanel, movePanel } from "./panels/layout";
  import { panelById, type DockId } from "./panels/registry";
  import Icon from "./Icon.svelte";

  export let dock: DockId;

  let menuFor: string | null = null;
  let menuX = 0;
  let menuY = 0;

  $: ids = panelsInDock($layout, dock);
  $: activeId = $layout.active[dock];
  $: activePanel = activeId ? panelById(activeId) : undefined;

  function onTabClick(id: string) {
    setActivePanel(dock, id);
  }

  function onTabContext(e: MouseEvent, id: string) {
    e.preventDefault();
    menuFor = id;
    menuX = e.clientX;
    menuY = e.clientY;
  }

  function moveTo(d: DockId) {
    if (menuFor) movePanel(menuFor, d);
    menuFor = null;
  }

  function closeMenu() {
    menuFor = null;
  }
</script>

<svelte:window on:click={closeMenu} />

<div class="dock" data-dock={dock}>
  <div class="tabbar">
    {#each ids as id}
      {@const p = panelById(id)}
      {#if p}
        <button
          class="tab"
          class:active={id === activeId}
          on:click={() => onTabClick(id)}
          on:contextmenu={(e) => onTabContext(e, id)}
        >
          <Icon icon={p.icon} size={12} />
          <span>{p.title}</span>
        </button>
      {/if}
    {/each}
  </div>

  <div class="body">
    {#if activePanel}
      <svelte:component this={activePanel.component} />
    {/if}
  </div>
</div>

{#if menuFor}
  <div
    class="ctx"
    role="menu"
    tabindex="-1"
    style:left="{menuX}px"
    style:top="{menuY}px"
    on:click|stopPropagation
    on:keydown|stopPropagation
  >
    <button on:click={() => moveTo("left")}>Move to Left Dock</button>
    <button on:click={() => moveTo("right")}>Move to Right Dock</button>
    <button on:click={() => moveTo("bottom")}>Move to Bottom Dock</button>
  </div>
{/if}

<style>
  .dock {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
    min-height: 0;
    background: var(--bg);
  }
  .tabbar {
    display: flex;
    align-items: center;
    height: 24px;
    background: var(--bg-subtle);
    border-bottom: 1px solid var(--border-subtle);
    padding: 0 var(--space-1);
    gap: 0;
    flex-shrink: 0;
    overflow: auto;
  }
  .tab {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    background: transparent;
    border: 0;
    border-bottom: 2px solid transparent;
    color: var(--text-muted);
    padding: 0 var(--space-2);
    height: 24px;
    font-size: var(--text-xs);
    font-family: var(--font-ui);
    cursor: pointer;
    white-space: nowrap;
  }
  .tab:hover {
    color: var(--text);
  }
  .tab.active {
    color: var(--text);
    border-bottom-color: var(--accent);
  }
  .body {
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .ctx {
    position: fixed;
    z-index: 1000;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 4px;
    display: flex;
    flex-direction: column;
    min-width: 180px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  }
  .ctx button {
    background: transparent;
    border: 0;
    color: var(--text);
    padding: 6px 10px;
    text-align: left;
    border-radius: var(--radius-sm);
    font-size: var(--text-sm);
    cursor: pointer;
  }
  .ctx button:hover {
    background: var(--bg-subtle);
  }
</style>
