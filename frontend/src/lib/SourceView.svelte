<script lang="ts">
  import { readFile, setBreakpoints } from "./store";

  export let sessionId: string | null;
  export let path: string = "";
  export let currentLine: number = 0;
  export let breakpoints: number[] = [];

  let text = "";
  let loaded = "";

  $: if (path && path !== loaded) {
    loaded = path;
    text = "";
    readFile(path)
      .then((t) => (text = t))
      .catch((e) => (text = "// error: " + e));
  }

  $: lines = text.split("\n");

  function toggle(line: number) {
    if (!sessionId || !path) return;
    const set = new Set(breakpoints);
    if (set.has(line)) set.delete(line);
    else set.add(line);
    const next = [...set].sort((a, b) => a - b);
    setBreakpoints(sessionId, path, next).catch(console.error);
  }
</script>

<div class="srcwrap">
  <div class="srcpath">{path || "(no source)"}</div>
  <div class="src">
    {#if !text}
      <div class="empty">Source loads when a session stops at a breakpoint.</div>
    {/if}
    {#each lines as line, i}
      {@const n = i + 1}
      {@const isBp = breakpoints.includes(n)}
      {@const isCur = currentLine === n}
      <div class="line {isCur ? 'cur' : ''}">
        <button class="gutter {isBp ? 'bp' : ''}" on:click={() => toggle(n)} title="Toggle breakpoint">
          {isBp ? "●" : ""}
        </button>
        <span class="lineno">{n}</span>
        <span class="code">{line}</span>
      </div>
    {/each}
  </div>
</div>

<style>
  .srcwrap { display:flex; flex-direction:column; height:100%; background:#1a1d23; }
  .srcpath { padding:6px 10px; background:#23272e; border-bottom:1px solid #30333a; font-size:12px; color:#9aa3b2; font-family: ui-monospace, monospace; }
  .src { flex:1; overflow:auto; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size:12px; padding:4px 0; }
  .empty { padding:12px; color:#6b7280; }
  .line { display:grid; grid-template-columns: 22px 44px 1fr; align-items:stretch; }
  .line.cur { background:#3a2e18; }
  .gutter { background:transparent; border:0; color:#e06050; cursor:pointer; padding:0; font-size:14px; }
  .gutter:hover:not(.bp) { color:#555; }
  .gutter.bp { color:#e06050; }
  .lineno { color:#5a6170; text-align:right; padding-right:10px; user-select:none; }
  .code { white-space:pre; color:#d7dae0; }
</style>
