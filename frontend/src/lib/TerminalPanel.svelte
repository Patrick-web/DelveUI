<script lang="ts">
  import { activeSessionId, sessionState, clearSessionOutput } from "./store";
  import TerminalPane from "./TerminalPane.svelte";
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";
  import { showInfo } from "./toast";

  $: output = $activeSessionId
    ? ($sessionState[$activeSessionId]?.output ?? [])
    : [];

  async function copy() {
    const text = output.map((l) => l.text).join("");
    await navigator.clipboard.writeText(text);
    showInfo("Copied", `${output.length} lines copied to clipboard`);
  }

  function clear() {
    if ($activeSessionId) clearSessionOutput($activeSessionId);
  }
</script>

<PanelHeader title="Terminal">
  <span class="dim">{output.length}</span>
  <button class="btn icon" title="Copy output" on:click={copy}>
    <Icon icon="solar:copy-linear" size={13} />
  </button>
  <button class="btn icon" title="Clear output" on:click={clear}>
    <Icon icon="solar:eraser-linear" size={13} />
  </button>
</PanelHeader>
<div class="body">
  <TerminalPane lines={output} />
</div>

<style>
  .body {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .dim {
    color: var(--text-faint);
    font-size: var(--text-xs);
    font-family: var(--font-mono);
  }
</style>
