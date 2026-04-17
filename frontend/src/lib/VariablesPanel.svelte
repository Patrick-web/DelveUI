<script lang="ts">
  import { activeSessionId, selectedFrameId } from "./store";
  import VariablesTree from "./VariablesTree.svelte";
  import PanelHeader from "./PanelHeader.svelte";
</script>

<PanelHeader title="Variables" />

<div class="body">
  {#if $activeSessionId && $selectedFrameId}
    <VariablesTree sessionId={$activeSessionId} frameId={$selectedFrameId} />
  {:else}
    <div class="empty">
      Variables appear when the program stops.<br />
      Hit a breakpoint or click <strong>Pause</strong> to break in.
    </div>
  {/if}
</div>

<style>
  .body { flex:1; min-height:0; overflow:hidden; display:flex; flex-direction:column; }
  .empty { padding:var(--space-3); color:var(--text-faint); font-size:var(--text-sm); }
</style>
