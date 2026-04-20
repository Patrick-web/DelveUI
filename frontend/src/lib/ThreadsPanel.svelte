<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";
  import { activeSessionId, sessions, activeSession } from "./store";
  import * as SessionService from "../../bindings/github.com/jp/DelveUI/internal/services/sessionservice";

  type Thread = { id: number; name: string };
  let threads: Thread[] = [];
  let err = "";
  let timer: any;

  function isLive(state?: string) {
    return state === "running" || state === "stopped" || state === "starting";
  }

  async function refresh() {
    if (!$activeSessionId || !isLive($activeSession?.state)) {
      threads = [];
      err = "";
      return;
    }
    try {
      const r = (await SessionService.Threads($activeSessionId)) as any;
      threads = r?.threads ?? [];
      err = "";
    } catch (e: any) {
      err = "";
      threads = [];
    }
  }

  onMount(() => {
    refresh();
    timer = setInterval(refresh, 2000);
  });
  onDestroy(() => clearInterval(timer));
  $: if ($activeSessionId || $sessions) refresh();

  export let hideHeader = false;
</script>

{#if !hideHeader}
<PanelHeader title="Threads">
  <button class="btn icon" title="Refresh" on:click={refresh}>
    <Icon icon="solar:refresh-linear" size={13} />
  </button>
</PanelHeader>
{/if}

<div class="body">
  {#if err}
    <div class="empty err">{err}</div>
  {:else if threads.length === 0}
    <div class="empty">
      {$activeSessionId ? "No threads yet." : "Start a session to see threads."}
    </div>
  {/if}
  {#each threads as t}
    <div class="thread">
      <span class="tid">#{t.id}</span>
      <span class="tname">{t.name}</span>
    </div>
  {/each}
</div>

<style>
  .body {
    flex: 1;
    overflow: auto;
  }
  .empty {
    padding: var(--space-3);
    color: var(--text-faint);
    font-size: var(--text-sm);
  }
  .err {
    color: var(--danger);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }
  .thread {
    display: flex;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-3);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }
  .thread:hover {
    background: var(--bg-subtle);
  }
  .tid {
    color: var(--text-faint);
    min-width: 48px;
  }
  .tname {
    color: var(--text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
