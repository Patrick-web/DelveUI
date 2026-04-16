<script lang="ts">
  import { fetchScopes, fetchVariables } from "./store";
  import type { Variable } from "./store";
  import Icon from "./Icon.svelte";
  import VariableNode from "./VariableNode.svelte";

  export let sessionId: string;
  export let frameId: number;

  type Scope = {
    name: string;
    variablesReference: number;
    expanded: boolean;
    children?: Variable[];
  };

  let scopes: Scope[] = [];
  let loadedFrame = -1;

  $: if (sessionId && frameId && frameId !== loadedFrame) {
    loadedFrame = frameId;
    load();
  }

  async function load() {
    try {
      const s = await fetchScopes(sessionId, frameId);
      scopes = (s?.scopes ?? []).map((sc: any) => ({
        name: sc.name,
        variablesReference: sc.variablesReference,
        expanded: sc.name === "Locals" || sc.name === "Arguments",
        children: undefined,
      }));
      for (const sc of scopes) {
        if (sc.expanded)
          sc.children = await fetchVariables(sessionId, sc.variablesReference);
      }
      scopes = scopes;
    } catch (e) {
      console.error(e);
    }
  }

  async function toggleScope(sc: Scope) {
    sc.expanded = !sc.expanded;
    if (sc.expanded && !sc.children)
      sc.children = await fetchVariables(sessionId, sc.variablesReference);
    scopes = scopes;
  }
</script>

<div class="vars">
  {#each scopes as sc (sc.name)}
    <div class="scope">
      <button class="row scope-row" on:click={() => toggleScope(sc)}>
        <span class="caret">
          <Icon
            icon={sc.expanded
              ? "solar:alt-arrow-down-linear"
              : "solar:alt-arrow-right-linear"}
            size={12}
          />
        </span>
        <span class="name">{sc.name}</span>
      </button>
      {#if sc.expanded && sc.children}
        {#each sc.children as v (v.name)}
          <VariableNode {sessionId} variable={v} depth={1} />
        {/each}
      {/if}
    </div>
  {/each}
</div>

<style>
  .vars {
    overflow: auto;
    flex: 1;
  }
  .scope {
    border-bottom: 1px solid var(--border-subtle);
  }
  .scope-row {
    width: 100%;
    text-align: left;
    background: transparent;
    border: 0;
    color: var(--text);
    padding: var(--space-1) var(--space-3);
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }
  .scope-row:hover {
    background: var(--bg-subtle);
  }
  .caret {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 14px;
    background: transparent;
    border: 0;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0;
  }
  .name {
    font-weight: 600;
    text-transform: uppercase;
    font-size: var(--text-xs);
    letter-spacing: 0.5px;
    color: var(--text-muted);
    font-family: var(--font-ui);
  }
</style>
