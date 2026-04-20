<script lang="ts">
  import { layout, toggleSection, type SidebarSectionId } from "./panels/layout";
  import Icon from "./Icon.svelte";

  export let id: SidebarSectionId;
  export let label: string;
  export let count: number | string | undefined = undefined;
  /** true: section body is allowed to grow and fill remaining space */
  export let flex: boolean = false;

  $: expanded = $layout.sidebarSections[id]?.expanded ?? false;
</script>

<section class="section" class:flex class:collapsed={!expanded}>
  <button
    class="sb-head"
    class:expanded
    aria-expanded={expanded}
    on:click={() => toggleSection(id)}
  >
    <span class="chev">
      <Icon icon="solar:alt-arrow-right-linear" size={10} />
    </span>
    <span class="label">{label}</span>
    {#if count !== undefined && count !== ""}
      <span class="count">{count}</span>
    {/if}
  </button>
  {#if expanded}
    <div class="sb-body">
      <slot />
    </div>
  {/if}
</section>

<style>
  .section {
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    min-height: 0;
  }
  .section.flex {
    flex: 1 1 auto;
    min-height: 26px;
  }
  .section.flex .sb-body {
    overflow: auto;
    flex: 1 1 auto;
    min-height: 0;
  }
  .sb-body {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
</style>
