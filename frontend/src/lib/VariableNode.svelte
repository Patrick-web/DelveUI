<script lang="ts">
  import Icon from "./Icon.svelte";
  import { fetchVariables } from "./store";
  import type { Variable } from "./store";
  import Self from "./VariableNode.svelte";

  export let sessionId: string;
  export let variable: Variable;
  export let depth: number = 1;

  let expanded = false;
  let children: Variable[] | null = null;

  async function toggle() {
    if (!variable.variablesReference) return;
    expanded = !expanded;
    if (expanded && !children) {
      children = await fetchVariables(sessionId, variable.variablesReference);
    }
  }
</script>

<div class="var" style:padding-left="{depth * 14 + 12}px">
  {#if variable.variablesReference}
    <button class="caret" on:click={toggle}>
      <Icon
        icon={expanded
          ? "solar:alt-arrow-down-linear"
          : "solar:alt-arrow-right-linear"}
        size={11}
      />
    </button>
  {:else}
    <span class="caret spacer"></span>
  {/if}
  <span class="vname">{variable.name}</span>
  {#if variable.type}<span class="vtype">{variable.type}</span>{/if}
  <span class="vval">{variable.value}</span>
</div>

{#if expanded && children}
  {#each children as child (child.name)}
    <Self {sessionId} variable={child} depth={depth + 1} />
  {/each}
{/if}

<style>
  .var {
    display: flex;
    gap: var(--space-2);
    padding-right: var(--space-3);
    color: var(--text);
    line-height: 20px;
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }
  .var:hover {
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
  .spacer {
    cursor: default;
  }
  .vname {
    color: var(--syn-fn);
  }
  .vtype {
    color: var(--text-faint);
  }
  .vval {
    color: var(--text);
    white-space: pre;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
