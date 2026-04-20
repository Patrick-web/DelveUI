<script lang="ts">
  import { sessions, activeSessionId, stopSession } from "./store";
  import SidebarSection from "./SidebarSection.svelte";
  import Icon from "./Icon.svelte";

  $: sessionList = Object.values($sessions);
  $: count = sessionList.length;

  function selectSession(id: string) {
    activeSessionId.set(id);
  }
</script>

<SidebarSection id="sessions" label="Sessions" count={count > 0 ? count : undefined}>
  {#if sessionList.length === 0}
    <div class="empty">No sessions. Click Run to start.</div>
  {:else}
    {#each sessionList as s (s.id)}
      <button
        class="sb-row"
        class:active={s.id === $activeSessionId}
        title={s.label}
        on:click={() => selectSession(s.id)}
      >
        <span class="state-dot state-{s.state}">●</span>
        <span class="sb-row-label">{s.label}</span>
        <button
          class="x"
          title="Stop & close"
          on:click|stopPropagation={() => stopSession(s.id)}
        >
          <Icon icon="solar:close-circle-linear" size={12} />
        </button>
      </button>
    {/each}
  {/if}
</SidebarSection>

<style>
  .empty {
    padding: 6px 12px 8px 22px;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .x {
    display: inline-flex;
    align-items: center;
    background: transparent;
    border: 0;
    color: inherit;
    opacity: 0;
    padding: 0;
    cursor: pointer;
    transition: opacity 80ms;
  }
  .sb-row:hover .x,
  .sb-row.active .x {
    opacity: 1;
  }
  .x:hover { color: var(--danger); }
  .sb-row.active .x:hover { color: #ffd6d6; }
</style>
