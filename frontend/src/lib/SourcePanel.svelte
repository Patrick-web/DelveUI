<script lang="ts">
  import { activeSessionId, sessionState, selectedFrame, selectedFrameId, setBreakpoints } from "./store";
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";
  import { readFile } from "./store";

  let text = "";
  let loadedPath = "";

  $: path = $selectedFrame?.source?.path ?? "";
  $: currentLine = $selectedFrame?.line ?? 0;
  $: breakpoints = $activeSessionId
    ? ($sessionState[$activeSessionId]?.breakpoints?.[path] ?? [])
    : [];

  $: if (path && path !== loadedPath) {
    loadedPath = path;
    text = "";
    readFile(path)
      .then((t) => (text = t))
      .catch((e) => (text = "// error: " + e));
  }

  $: lines = text.split("\n");

  function toggle(line: number) {
    if (!$activeSessionId || !path) return;
    const set = new Set(breakpoints);
    if (set.has(line)) set.delete(line);
    else set.add(line);
    const next = [...set].sort((a, b) => a - b);
    setBreakpoints($activeSessionId, path, next).catch(console.error);
  }

  function shortPath(p: string) {
    if (!p) return "(no source)";
    return p.split("/").slice(-3).join("/");
  }

  // Auto-scroll to current line
  let container: HTMLDivElement;
  $: if (currentLine && container) {
    requestAnimationFrame(() => {
      const lineEl = container?.querySelector(`[data-line="${currentLine}"]`);
      lineEl?.scrollIntoView({ block: "center", behavior: "smooth" });
    });
  }
</script>

<PanelHeader title="Source">
  {#if path}
    <span class="path-hint">{shortPath(path)}</span>
  {/if}
</PanelHeader>

<div class="src" bind:this={container}>
  {#if !text && !path}
    <div class="empty">
      <Icon icon="solar:code-2-bold-duotone" size={24} color="var(--text-faint)" />
      <span>Source appears when the program stops at a breakpoint.</span>
    </div>
  {:else if !text && path}
    <div class="empty">Loading {shortPath(path)}…</div>
  {:else}
    {#each lines as line, i}
      {@const n = i + 1}
      {@const isBp = breakpoints.includes(n)}
      {@const isCur = currentLine === n}
      <div class="line" class:cur={isCur} data-line={n}>
        <button class="gutter" class:bp={isBp} on:click={() => toggle(n)} title="Toggle breakpoint">
          {#if isBp}
            <Icon icon="solar:record-circle-bold" size={11} color="var(--danger)" />
          {:else}
            <span class="gutter-space"></span>
          {/if}
        </button>
        <span class="lineno">{n}</span>
        <span class="code">{line}</span>
      </div>
    {/each}
  {/if}
</div>

<style>
  .path-hint { font-family:var(--font-mono); font-size:var(--text-xs); color:var(--text-faint); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .src { flex:1; overflow:auto; font-family:var(--font-mono); font-size:var(--text-sm); padding:var(--space-1) 0; background:var(--bg-subtle); }
  .empty { display:flex; flex-direction:column; align-items:center; gap:var(--space-2); padding:var(--space-8) var(--space-4); color:var(--text-faint); font-size:var(--text-sm); text-align:center; font-family:var(--font-ui); }
  .line { display:grid; grid-template-columns:22px 44px 1fr; align-items:stretch; min-height:20px; }
  .line.cur { background:rgba(91,135,214,0.12); }
  .gutter { background:transparent; border:0; cursor:pointer; padding:0; display:flex; align-items:center; justify-content:center; min-width:22px; }
  .gutter:hover:not(.bp) { background:rgba(224,108,117,0.1); }
  .gutter-space { width:11px; height:11px; }
  .lineno { color:var(--text-faint); text-align:right; padding-right:var(--space-2); user-select:none; font-size:var(--text-xs); line-height:20px; }
  .code { white-space:pre; color:var(--text); line-height:20px; padding-right:var(--space-3); }
</style>
