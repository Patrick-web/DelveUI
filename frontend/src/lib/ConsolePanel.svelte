<script lang="ts">
  import { activeSession, activeSessionId, sessionState, evaluate } from "./store";
  import { showInfo } from "./toast";
  import TerminalPane from "./TerminalPane.svelte";
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";

  let expr = "";
  let history: { cat: string; text: string }[] = [];
  let pastCmds: string[] = [];
  let histIdx = -1;

  $: output = $activeSessionId
    ? ($sessionState[$activeSessionId]?.output ?? [])
    : [];
  $: stack = $activeSessionId
    ? ($sessionState[$activeSessionId]?.stack ?? [])
    : [];
  $: frameId = stack[0]?.id ?? 0;

  function isConsole(cat: string) {
    return (
      cat === "console" ||
      cat === "telemetry" ||
      cat === "important" ||
      cat.startsWith("repl")
    );
  }

  $: lines = [...output.filter((l) => isConsole(l.cat)), ...history];

  async function run() {
    if (!$activeSessionId || !expr) return;
    const e = expr;
    expr = "";
    pastCmds = [...pastCmds, e];
    histIdx = -1;
    history = [...history, { cat: "repl-in", text: "> " + e + "\n" }];
    try {
      const r = await evaluate($activeSessionId, e, frameId);
      const txt = (r?.result ?? "") + (r?.type ? "  : " + r.type : "");
      history = [...history, { cat: "repl-out", text: txt + "\n" }];
    } catch (err: any) {
      history = [...history, { cat: "stderr", text: String(err) + "\n" }];
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Enter") {
      run();
    } else if (e.key === "ArrowUp") {
      if (!pastCmds.length) return;
      e.preventDefault();
      histIdx = histIdx < 0 ? pastCmds.length - 1 : Math.max(0, histIdx - 1);
      expr = pastCmds[histIdx] ?? "";
    } else if (e.key === "ArrowDown") {
      if (histIdx < 0) return;
      e.preventDefault();
      histIdx = histIdx + 1;
      if (histIdx >= pastCmds.length) {
        histIdx = -1;
        expr = "";
      } else {
        expr = pastCmds[histIdx];
      }
    }
  }
</script>

<PanelHeader title="Debug Console">
  <button class="btn icon" title="Copy" on:click={async () => {
    const text = lines.map(l => l.text).join("");
    await navigator.clipboard.writeText(text);
    showInfo("Copied", "Console output copied");
  }}>
    <Icon icon="solar:copy-linear" size={13} />
  </button>
  <button class="btn icon" title="Clear" on:click={() => (history = [])}>
    <Icon icon="solar:eraser-linear" size={13} />
  </button>
</PanelHeader>

<div class="body">
  <TerminalPane {lines} />
  <div class="repl">
    <span class="prompt">&gt;</span>
    <input
      class="tx input"
      bind:value={expr}
      placeholder={$activeSession ? "evaluate expression" : "start a session to evaluate"}
      on:keydown={onKey}
      disabled={!$activeSession}
    />
  </div>
</div>

<style>
  .body {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-subtle);
  }
  .repl {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border-top: 1px solid var(--border-subtle);
    background: var(--bg);
  }
  .prompt {
    color: var(--success);
    font-family: var(--font-mono);
  }
  .input {
    flex: 1;
    background: transparent;
    border: 0;
    height: 22px;
    padding: 0;
    font-family: var(--font-mono);
  }
  .input:focus {
    box-shadow: none;
  }
</style>
