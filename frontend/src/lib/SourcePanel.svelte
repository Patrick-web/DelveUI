<script lang="ts">
  import { activeSessionId, activeSession, sessionState, selectedFrame, selectedFrameId, manualSourcePath, setBreakpoints, globalBreakpoints, fetchVariables, fetchScopes } from "./store";
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";
  import { readFile } from "./store";

  let text = "";
  let loadedPath = "";

  $: framePath = $selectedFrame?.source?.path ?? "";
  $: path = $manualSourcePath || framePath;
  $: currentLine = ($manualSourcePath && $manualSourcePath !== framePath) ? 0 : ($selectedFrame?.line ?? 0);

  // Read breakpoints from session state if session exists, otherwise from global store
  $: breakpoints = (() => {
    if ($activeSessionId && $sessionState[$activeSessionId]?.breakpoints?.[path]?.length) {
      return $sessionState[$activeSessionId].breakpoints[path];
    }
    return $globalBreakpoints[path] ?? [];
  })();

  // Inline variable values when stopped at current line
  let inlineVars: Record<number, string> = {};
  $: if ($activeSession?.state === "stopped" && currentLine && $activeSessionId && $selectedFrameId) {
    loadInlineVars();
  } else {
    inlineVars = {};
  }

  async function loadInlineVars() {
    if (!$activeSessionId || !$selectedFrameId) return;
    try {
      const scopes = await fetchScopes($activeSessionId, $selectedFrameId) as any;
      const locals = scopes?.scopes?.find((s: any) => s.name === "Locals");
      if (!locals) return;
      const vars = await fetchVariables($activeSessionId, locals.variablesReference);
      const map: Record<number, string> = {};
      if (currentLine) {
        const parts = vars.map((v) => `${v.name} = ${v.value}`).slice(0, 6);
        if (parts.length) map[currentLine] = parts.join("  ·  ");
      }
      inlineVars = map;
    } catch { inlineVars = {}; }
  }

  $: if (path && path !== loadedPath) {
    loadedPath = path;
    text = "";
    readFile(path)
      .then((t) => (text = t))
      .catch((e) => (text = "// error: " + e));
  }

  $: lines = text.split("\n");

  function toggleBreakpoint(line: number) {
    if (!path) return;
    const set = new Set(breakpoints);
    if (set.has(line)) set.delete(line);
    else set.add(line);
    const next = [...set].sort((a, b) => a - b);
    setBreakpoints($activeSessionId, path, next).catch(console.error);
  }

  // Right-click context menu
  let ctxLine = 0;
  let ctxX = 0;
  let ctxY = 0;
  let ctxOpen = false;
  let conditionInput = "";
  let logInput = "";
  let hitInput = "";
  let editingType: "condition" | "log" | "hit" | null = null;

  function onGutterContext(e: MouseEvent, line: number) {
    e.preventDefault();
    ctxLine = line;
    ctxX = e.clientX;
    ctxY = e.clientY;
    ctxOpen = true;
    editingType = null;
  }

  function closeCtx() { ctxOpen = false; editingType = null; }

  function startEdit(type: "condition" | "log" | "hit") {
    editingType = type;
    conditionInput = "";
    logInput = "";
    hitInput = "";
  }

  function addBreakpointFromCtx() {
    toggleBreakpoint(ctxLine);
    closeCtx();
  }

  function removeBreakpointFromCtx() {
    if (!path) return;
    const next = breakpoints.filter((l) => l !== ctxLine);
    setBreakpoints($activeSessionId, path, next).catch(console.error);
    closeCtx();
  }

  function applyEdit() {
    if (!path) { closeCtx(); return; }
    // Ensure the breakpoint exists on this line
    if (!breakpoints.includes(ctxLine)) {
      const next = [...breakpoints, ctxLine].sort((a, b) => a - b);
      setBreakpoints($activeSessionId, path, next).catch(console.error);
    }
    // TODO: When enhanced breakpoint store supports conditions, pass them here
    // For now the breakpoint is set; condition/log/hit stored as UI feedback
    const { showInfo } = (() => { try { return require("./toast"); } catch { return { showInfo: () => {} }; } })();
    if (editingType === "condition" && conditionInput) {
      console.log(`Condition breakpoint on line ${ctxLine}: ${conditionInput}`);
    } else if (editingType === "log" && logInput) {
      console.log(`Logpoint on line ${ctxLine}: ${logInput}`);
    } else if (editingType === "hit" && hitInput) {
      console.log(`Hit count breakpoint on line ${ctxLine}: ${hitInput}`);
    }
    closeCtx();
  }

  function shortPath(p: string) {
    if (!p) return "(no source)";
    return p.split("/").slice(-3).join("/");
  }

  let container: HTMLDivElement;
  $: if (currentLine && container) {
    requestAnimationFrame(() => {
      const lineEl = container?.querySelector(`[data-line="${currentLine}"]`);
      lineEl?.scrollIntoView({ block: "center", behavior: "smooth" });
    });
  }
</script>

<svelte:window on:click={() => ctxOpen && closeCtx()} />

<PanelHeader title="Source">
  {#if path}
    <span class="path-hint">{shortPath(path)}</span>
  {/if}
</PanelHeader>

<div class="src" bind:this={container}>
  {#if !text && !path}
    <div class="empty">
      <Icon icon="solar:code-2-bold-duotone" size={24} color="var(--text-faint)" />
      <span>Open a file from the Files panel or press <strong>⌘O</strong></span>
    </div>
  {:else if !text && path}
    <div class="empty">Loading {shortPath(path)}…</div>
  {:else}
    {#each lines as line, i}
      {@const n = i + 1}
      {@const isBp = breakpoints.includes(n)}
      {@const isCur = currentLine === n}
      <div class="line" class:cur={isCur} data-line={n}>
        <button
          class="gutter"
          class:bp={isBp}
          on:click={() => toggleBreakpoint(n)}
          on:contextmenu={(e) => onGutterContext(e, n)}
          title="Click: toggle breakpoint · Right-click: options"
        >
          {#if isBp}
            <Icon icon="solar:record-circle-bold" size={11} color="var(--danger)" />
          {:else}
            <span class="gutter-space"></span>
          {/if}
        </button>
        <span class="lineno">{n}</span>
        <span class="code">{line}</span>
        {#if inlineVars[n]}
          <span class="inline-vars">{inlineVars[n]}</span>
        {/if}
      </div>
    {/each}
  {/if}
</div>

<!-- Gutter context menu -->
{#if ctxOpen}
  <div class="ctx" style:left="{ctxX}px" style:top="{ctxY}px" role="menu" tabindex="-1" on:click|stopPropagation on:keydown|stopPropagation>
    {#if !editingType}
      {#if breakpoints.includes(ctxLine)}
        <button on:click={removeBreakpointFromCtx}>Remove Breakpoint</button>
      {:else}
        <button on:click={addBreakpointFromCtx}>Add Breakpoint</button>
      {/if}
      <button on:click={() => startEdit("condition")}>Add Condition…</button>
      <button on:click={() => startEdit("log")}>Add Log Message…</button>
      <button on:click={() => startEdit("hit")}>Hit Count…</button>
    {:else}
      <div class="edit-row">
        <span class="edit-label">
          {editingType === "condition" ? "Condition:" : editingType === "log" ? "Log message:" : "Hit count:"}
        </span>
        {#if editingType === "condition"}
          <!-- svelte-ignore a11y-autofocus -->
          <input class="edit-input" autofocus bind:value={conditionInput} placeholder="e.g. i > 10" on:keydown={(e) => { if (e.key === "Enter") applyEdit(); if (e.key === "Escape") closeCtx(); }} />
        {:else if editingType === "log"}
          <!-- svelte-ignore a11y-autofocus -->
          <input class="edit-input" autofocus bind:value={logInput} placeholder='e.g. "value is x"' on:keydown={(e) => { if (e.key === "Enter") applyEdit(); if (e.key === "Escape") closeCtx(); }} />
        {:else}
          <!-- svelte-ignore a11y-autofocus -->
          <input class="edit-input" autofocus bind:value={hitInput} placeholder="e.g. 5" on:keydown={(e) => { if (e.key === "Enter") applyEdit(); if (e.key === "Escape") closeCtx(); }} />
        {/if}
        <button class="btn primary" style="font-size:11px;padding:2px 8px" on:click={applyEdit}>Set</button>
      </div>
    {/if}
  </div>
{/if}

<style>
  .path-hint { font-family:var(--font-mono); font-size:var(--text-xs); color:var(--text-faint); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .src { flex:1; overflow:auto; font-family:var(--font-mono); font-size:var(--text-sm); padding:var(--space-1) 0; background:var(--bg-subtle); }
  .empty { display:flex; flex-direction:column; align-items:center; gap:var(--space-2); padding:var(--space-8) var(--space-4); color:var(--text-faint); font-size:var(--text-sm); text-align:center; font-family:var(--font-ui); }
  .line { display:grid; grid-template-columns:22px 44px 1fr auto; align-items:stretch; min-height:20px; }
  .line.cur { background:rgba(91,135,214,0.12); }
  .gutter { background:transparent; border:0; cursor:pointer; padding:0; display:flex; align-items:center; justify-content:center; min-width:22px; }
  .gutter:hover:not(.bp) { background:rgba(224,108,117,0.1); }
  .gutter-space { width:11px; height:11px; }
  .lineno { color:var(--text-faint); text-align:right; padding-right:var(--space-2); user-select:none; font-size:var(--text-xs); line-height:20px; }
  .code { white-space:pre; color:var(--text); line-height:20px; padding-right:var(--space-3); }
  .inline-vars {
    color:var(--text-faint); font-size:var(--text-xs); line-height:20px;
    padding:0 var(--space-2); white-space:nowrap; overflow:hidden;
    text-overflow:ellipsis; opacity:0.7; font-style:italic;
  }
  .ctx {
    position:fixed; z-index:200; background:var(--bg-elevated);
    border:1px solid var(--border); border-radius:var(--radius-sm);
    padding:4px; min-width:200px; box-shadow:0 8px 24px rgba(0,0,0,0.4);
  }
  .ctx button {
    display:block; width:100%; background:transparent; border:0;
    color:var(--text); padding:6px 10px; text-align:left;
    border-radius:var(--radius-sm); font-size:var(--text-sm);
    cursor:pointer; font-family:var(--font-ui);
  }
  .ctx button:hover { background:var(--bg-subtle); }
  .edit-row { padding:6px 8px; display:flex; flex-direction:column; gap:4px; }
  .edit-label { font-size:var(--text-xs); color:var(--text-muted); font-weight:600; }
  .edit-input {
    background:var(--bg-subtle); border:1px solid var(--border); color:var(--text);
    padding:4px 8px; border-radius:var(--radius-sm); font-family:var(--font-mono);
    font-size:var(--text-sm); outline:none;
  }
  .edit-input:focus { border-color:var(--accent); }
</style>
