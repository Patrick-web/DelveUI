<script lang="ts">
  import { onDestroy, tick } from "svelte";
  import { EditorView, gutter, GutterMarker, lineNumbers, highlightActiveLine, keymap, Decoration, type DecorationSet } from "@codemirror/view";
  import { EditorState, StateField, StateEffect, RangeSet, Compartment } from "@codemirror/state";
  import { go } from "@codemirror/lang-go";
  import { editorTheme, editorHighlighting } from "./editor-theme";
  import { vim, Vim } from "@replit/codemirror-vim";
  import { search, openSearchPanel, searchKeymap } from "@codemirror/search";
  import { defaultKeymap } from "@codemirror/commands";
  import { activeSessionId, activeSession, sessionState, selectedFrame, selectedFrameId, manualSourcePath, scrollToLineRequest, setBreakpoints, globalBreakpoints, fetchVariables, fetchScopes, workspace } from "./store";
  import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";
  import { showInfo, showError } from "./toast";
  import { appSettings } from "./settings-store";
  import Icon from "./Icon.svelte";
  import SymbolPicker from "./SymbolPicker.svelte";
  import { readFile } from "./store";
  import { extractGoSymbols, enclosingFunction, type GoSymbol } from "./go-symbols";

  let editorEl: HTMLDivElement;
  let view: EditorView | null = null;
  let loadedPath = "";
  let dirty = false;

  $: framePath = $selectedFrame?.source?.path ?? "";
  $: path = $manualSourcePath || framePath;
  $: currentLine = ($manualSourcePath && $manualSourcePath !== framePath) ? 0 : ($selectedFrame?.line ?? 0);
  // Always read from global breakpoints (synced with session on start)
  $: bpLines = $globalBreakpoints[path] ?? [];

  // Load file when path changes
  $: if (path && path !== loadedPath) {
    loadedPath = path;
    loadFile(path);
  }

  // Update decorations when breakpoints or current line change
  // Use bpLines and currentLine directly so Svelte tracks them as dependencies
  $: bpLines, currentLine, view && updateDecorations(bpLines, currentLine);

  // Scroll to current line when stopped
  $: if (view && currentLine > 0) scrollToLine(currentLine);

  // Scroll to requested line (from breakpoints panel click)
  $: if (view && $scrollToLineRequest > 0) {
    scrollToLine($scrollToLineRequest);
    scrollToLineRequest.set(0);
  }

  async function loadFile(filePath: string) {
    try {
      const text = await readFile(filePath);
      await tick();
      createEditor(text);
      dirty = false;
    } catch (e) {
      await tick();
      createEditor("// Error loading file: " + e);
      dirty = false;
    }
  }

  async function saveFile() {
    if (!view || !path || !dirty) return;
    const content = view.state.doc.toString();
    try {
      await FileService.WriteFile(path, content);
      dirty = false;
      showInfo("Saved", shortPath(path));
    } catch (e: any) {
      showError("Save failed", String(e?.message ?? e));
    }
  }

  // --- Breakpoint gutter ---
  const breakpointEffect = StateEffect.define<{ pos: number; on: boolean }>();

  const breakpointState = StateField.define<RangeSet<GutterMarker>>({
    create() { return RangeSet.empty; },
    update(set, tr) {
      set = set.map(tr.changes);
      for (const e of tr.effects) {
        if (e.is(breakpointEffect)) {
          if (e.value.on) {
            set = set.update({ add: [breakpointMarker.range(e.value.pos)] });
          } else {
            set = set.update({ filter: (from) => from !== e.value.pos });
          }
        } else if (e.is(setAllBreakpoints)) {
          set = RangeSet.of(e.value.map((pos: number) => breakpointMarker.range(pos)));
        }
      }
      return set;
    },
  });

  const setAllBreakpoints = StateEffect.define<number[]>();

  // Current stopped line highlight
  const setCurrentLine = StateEffect.define<number>(); // pos or -1 to clear

  const currentLineDecor = Decoration.line({ class: "cm-stopped-line" });

  const currentLineField = StateField.define<DecorationSet>({
    create() { return Decoration.none; },
    update(set, tr) {
      for (const e of tr.effects) {
        if (e.is(setCurrentLine)) {
          if (e.value >= 0) {
            return Decoration.set([currentLineDecor.range(e.value)]);
          }
          return Decoration.none;
        }
      }
      return set.map(tr.changes);
    },
    provide: (f) => EditorView.decorations.from(f),
  });

  class BreakpointMarker extends GutterMarker {
    toDOM() {
      const el = document.createElement("span");
      el.className = "cm-bp-mark";
      el.textContent = "●";
      return el;
    }
  }
  const breakpointMarker = new BreakpointMarker();

  // Ghost marker rendered on every line without a real breakpoint. CSS keeps
  // it invisible until the gutter cell (or the matching line-number cell)
  // is hovered — that's where the "click to add" hint comes from.
  class GhostBreakpointMarker extends GutterMarker {
    toDOM() {
      const el = document.createElement("span");
      el.className = "cm-bp-ghost";
      el.textContent = "●";
      return el;
    }
  }
  const ghostBreakpointMarker = new GhostBreakpointMarker();

  const breakpointGutter = gutter({
    class: "cm-breakpoint-gutter",
    markers: (v) => v.state.field(breakpointState),
    lineMarker(view, line) {
      const bps = view.state.field(breakpointState);
      let hasBp = false;
      bps.between(line.from, line.from, () => {
        hasBp = true;
        return false;
      });
      return hasBp ? null : ghostBreakpointMarker;
    },
    initialSpacer: () => breakpointMarker,
    domEventHandlers: {
      mousedown(view, line) {
        toggleBreakpointAtLine(view, line.from);
        return true;
      },
      contextmenu(view, line, event) {
        const lineNo = view.state.doc.lineAt(line.from).number;
        onGutterContext(event as MouseEvent, lineNo);
        return true;
      },
    },
  });

  function toggleBreakpointAtLine(view: EditorView, pos: number) {
    const lineNo = view.state.doc.lineAt(pos).number;
    const set = new Set(bpLines);
    if (set.has(lineNo)) set.delete(lineNo);
    else set.add(lineNo);
    const next = [...set].sort((a, b) => a - b);
    setBreakpoints($activeSessionId, path, next).catch(console.error);
  }

  // --- Current line highlight ---
  const currentLineDecoration = EditorView.decorations.compute(["doc"], () => {
    return RangeSet.empty;
  });

  // --- Vim commands ---
  let vimCommandsRegistered = false;
  function registerVimCommands() {
    if (vimCommandsRegistered) return;
    vimCommandsRegistered = true;
    Vim.defineEx("write", "w", () => { saveFile(); });
    Vim.defineEx("quit", "q", () => { /* no-op, can't close panel */ });
    Vim.defineEx("wq", "wq", () => { saveFile(); });
  }

  // --- Vim + Search ---
  const vimCompartment = new Compartment();
  $: vimEnabled = $appSettings.vimMode ?? false;
  $: if (view) {
    view.dispatch({ effects: vimCompartment.reconfigure(vimEnabled ? vim() : []) });
  }

  // --- Editor creation ---
  function createEditor(text: string) {
    if (view) { view.destroy(); view = null; }
    if (!editorEl) {
      // editorEl is always rendered (just hidden when path is empty), but
      // guard anyway. Retry after the next tick — the bind should be in.
      tick().then(() => { if (editorEl) createEditor(text); });
      return;
    }

    const extensions = [
      vimCompartment.of(vimEnabled ? vim() : []),
      go(),
      editorTheme,
      editorHighlighting,
      keymap.of([{
        key: "Mod-s",
        run: () => { saveFile(); return true; },
      }]),
      EditorView.updateListener.of((u) => {
        if (u.docChanged) dirty = true;
        if (u.docChanged || u.selectionSet) scheduleSymbolRefresh();
      }),
      lineNumbers({
        domEventHandlers: {
          mousedown(view, line) {
            toggleBreakpointAtLine(view, line.from);
            return true;
          },
          contextmenu(view, line, event) {
            const lineNo = view.state.doc.lineAt(line.from).number;
            onGutterContext(event as MouseEvent, lineNo);
            return true;
          },
        },
      }),
      highlightActiveLine(),
      search({ top: true }),
      keymap.of(searchKeymap),
      breakpointGutter,
      breakpointState,
      currentLineField,
      EditorView.theme({
        "&": { height: "100%", fontSize: `${$appSettings.bufferFontSize ?? 13}px` },
        ".cm-content": { fontFamily: "var(--font-mono)", padding: "0" },
        ".cm-gutters": {
          backgroundColor: "var(--bg-subtle)",
          borderRight: "1px solid var(--border-subtle)",
          color: "var(--text-faint)",
          minWidth: "44px",
        },
        ".cm-breakpoint-gutter .cm-gutterElement": {
          padding: "0 2px",
          cursor: "pointer",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          minWidth: "20px",
        },
        ".cm-bp-mark": {
          color: "var(--danger)",
          fontSize: "14px",
          lineHeight: "1",
        },
        ".cm-bp-ghost": {
          color: "var(--danger)",
          fontSize: "14px",
          lineHeight: "1",
          opacity: "0",
          transition: "opacity 80ms ease-out",
        },
        ".cm-breakpoint-gutter .cm-gutterElement:hover .cm-bp-ghost, .cm-breakpoint-gutter .cm-gutterElement.cm-bp-hover .cm-bp-ghost": {
          opacity: "0.35",
        },
        ".cm-stopped-line": { backgroundColor: "rgba(229,192,123,0.15)", borderLeft: "2px solid var(--warning)" },
        ".cm-activeLine": { backgroundColor: "rgba(91,135,214,0.08)" },
        ".cm-activeLineGutter": { backgroundColor: "rgba(91,135,214,0.08)" },
        "&.cm-focused": { outline: "none" },
        ".cm-line": { padding: "0 4px 0 0" },
        ".cm-scroller": { overflow: "auto" },
        ".cm-selectionBackground": { backgroundColor: "rgba(91,135,214,0.3) !important" },
        "&.cm-focused .cm-selectionBackground": { backgroundColor: "rgba(91,135,214,0.4) !important" },
        ".cm-fat-cursor": { backgroundColor: "var(--accent) !important", color: "var(--bg) !important" },
        ".cm-cursor": { borderLeftColor: "var(--accent)" },
        /* Search panel styling */
        ".cm-search": {
          background: "var(--bg-elevated)",
          borderBottom: "1px solid var(--border)",
          padding: "4px 8px",
          fontSize: "12px",
          fontFamily: "var(--font-mono)",
        },
        ".cm-search input, .cm-search button": {
          background: "var(--bg-subtle)",
          border: "1px solid var(--border)",
          color: "var(--text)",
          borderRadius: "3px",
          padding: "2px 6px",
          fontSize: "12px",
          fontFamily: "var(--font-mono)",
        },
        ".cm-search button:hover": { background: "var(--bg)" },
        ".cm-search label": { color: "var(--text-muted)", fontSize: "11px" },
        ".cm-searchMatch": { backgroundColor: "rgba(255,204,0,0.25)", borderRadius: "2px" },
        ".cm-searchMatch-selected": { backgroundColor: "rgba(255,204,0,0.5)" },
      }),
    ];

    view = new EditorView({
      parent: editorEl,
      state: EditorState.create({ doc: text, extensions }),
    });

    registerVimCommands();

    // Apply initial breakpoints + current line
    updateDecorations(bpLines, currentLine);
    if (currentLine > 0) scrollToLine(currentLine);

    // Initial symbol scan (tree takes a microtask to be available)
    scheduleSymbolRefresh();

    wireGutterHoverBridge(view);
  }

  // Hovering over a line number cell should also light up the breakpoint
  // gutter cell on the same line so the ghost dot appears as an "add me" hint.
  // The two gutters are CodeMirror siblings with parallel children, so we can
  // forward the hover by index.
  function wireGutterHoverBridge(v: EditorView) {
    const root = v.dom;
    let hovered: HTMLElement | null = null;
    function clear() {
      if (hovered) { hovered.classList.remove("cm-bp-hover"); hovered = null; }
    }
    root.addEventListener("mousemove", (e) => {
      const t = e.target as HTMLElement;
      const lineCell = t.closest?.(".cm-lineNumbers .cm-gutterElement") as HTMLElement | null;
      if (!lineCell) { clear(); return; }
      const idx = Array.prototype.indexOf.call(lineCell.parentElement!.children, lineCell);
      const bpGutter = root.querySelector(".cm-breakpoint-gutter") as HTMLElement | null;
      const next = bpGutter?.children?.[idx] as HTMLElement | undefined;
      if (next === hovered) return;
      clear();
      if (next) { next.classList.add("cm-bp-hover"); hovered = next; }
    });
    root.addEventListener("mouseleave", clear);
  }

  function updateDecorations(lines: number[], curLine: number) {
    if (!view) return;
    const doc = view.state.doc;

    const effects: StateEffect<any>[] = [];

    // Set breakpoints
    const positions = lines
      .filter((l) => l > 0 && l <= doc.lines)
      .map((l) => doc.line(l).from);
    effects.push(setAllBreakpoints.of(positions));

    // Set current stopped line
    if (curLine > 0 && curLine <= doc.lines) {
      effects.push(setCurrentLine.of(doc.line(curLine).from));
    } else {
      effects.push(setCurrentLine.of(-1));
    }

    view.dispatch({ effects });
  }

  function scrollToLine(line: number) {
    if (!view) return;
    const doc = view.state.doc;
    if (line < 1 || line > doc.lines) return;
    const pos = doc.line(line).from;
    view.dispatch({
      effects: EditorView.scrollIntoView(pos, { y: "center" }),
    });
  }

  // --- Right-click context menu ---
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
    conditionInput = ""; logInput = ""; hitInput = "";
  }

  function addBreakpointFromCtx() {
    if (!path) return;
    const next = [...new Set([...bpLines, ctxLine])].sort((a, b) => a - b);
    setBreakpoints($activeSessionId, path, next).catch(console.error);
    closeCtx();
  }

  function removeBreakpointFromCtx() {
    if (!path) return;
    const next = bpLines.filter((l) => l !== ctxLine);
    setBreakpoints($activeSessionId, path, next).catch(console.error);
    closeCtx();
  }

  function applyEdit() {
    if (!path) { closeCtx(); return; }
    if (!bpLines.includes(ctxLine)) {
      const next = [...bpLines, ctxLine].sort((a, b) => a - b);
      setBreakpoints($activeSessionId, path, next).catch(console.error);
    }
    closeCtx();
  }

  function shortPath(p: string) {
    if (!p) return "(no source)";
    return p.split("/").slice(-3).join("/");
  }

  // Inline variable values
  let inlineVarText = "";
  $: if ($activeSession?.state === "stopped" && currentLine && $activeSessionId && $selectedFrameId) {
    loadInlineVars();
  } else {
    inlineVarText = "";
  }

  async function loadInlineVars() {
    if (!$activeSessionId || !$selectedFrameId) return;
    try {
      const scopes = await fetchScopes($activeSessionId, $selectedFrameId) as any;
      const locals = scopes?.scopes?.find((s: any) => s.name === "Locals");
      if (!locals) return;
      const vars = await fetchVariables($activeSessionId, locals.variablesReference);
      inlineVarText = vars.map((v) => `${v.name} = ${v.value}`).slice(0, 6).join("  ·  ");
    } catch { inlineVarText = ""; }
  }

  // --- Symbols + breadcrumb ---
  let symbols: GoSymbol[] = [];
  let currentFunc: GoSymbol | null = null;
  let symbolPickerOpen = false;

  // Derive a friendly relative path: strip the workspace root or the
  // active config's cwd/program prefix if present.
  $: relPath = (() => {
    if (!path) return "";
    const root = $workspace?.root ?? "";
    if (root && path.startsWith(root + "/")) return path.slice(root.length + 1);
    const cfg = $activeSession?.cfg as any;
    for (const base of [cfg?.cwd, cfg?.program].filter(Boolean)) {
      const b = String(base).replace(/\/+$/, "");
      if (path.startsWith(b + "/")) return path.slice(b.length + 1);
    }
    // fall back to the last three segments
    return path.split("/").slice(-3).join("/");
  })();
  $: pathSegments = relPath ? relPath.split("/") : [];

  function refreshSymbols() {
    if (!view) { symbols = []; currentFunc = null; return; }
    try {
      symbols = extractGoSymbols(view.state);
      const pos = view.state.selection.main.head;
      currentFunc = enclosingFunction(view.state, pos);
    } catch {
      symbols = [];
      currentFunc = null;
    }
  }

  let symbolRefreshTimer: any = null;
  function scheduleSymbolRefresh() {
    if (symbolRefreshTimer) return;
    symbolRefreshTimer = setTimeout(() => {
      symbolRefreshTimer = null;
      refreshSymbols();
    }, 120);
  }

  function gotoSymbol(sym: GoSymbol) {
    if (!view) return;
    const doc = view.state.doc;
    if (sym.line < 1 || sym.line > doc.lines) return;
    const lineInfo = doc.line(sym.line);
    view.dispatch({
      selection: { anchor: lineInfo.from },
      effects: EditorView.scrollIntoView(lineInfo.from, { y: "center" }),
    });
    view.focus();
  }

  function openSymbolPicker() {
    refreshSymbols();
    symbolPickerOpen = true;
  }

  function openFindPanel() {
    if (!view) return;
    openSearchPanel(view);
  }

  // Cmd+Shift+O while editor is focused → open symbol picker
  function onPanelKey(e: KeyboardEvent) {
    const mod = e.metaKey || e.ctrlKey;
    if (mod && e.shiftKey && e.key.toLowerCase() === "o") {
      e.preventDefault();
      openSymbolPicker();
    }
  }

  onDestroy(() => { if (view) { view.destroy(); view = null; } });
</script>

<svelte:window on:click={() => ctxOpen && closeCtx()} />

<!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
<div class="sp-root" role="region" on:keydown={onPanelKey}>
  {#if path}
    <div class="breadcrumb">
      <div class="crumbs">
        {#each pathSegments as seg, i}
          {#if i > 0}
            <Icon icon="solar:alt-arrow-right-linear" size={10} color="var(--text-faint)" />
          {/if}
          <span class="crumb" class:leaf={i === pathSegments.length - 1}>{seg}</span>
        {/each}
        {#if currentFunc}
          <Icon icon="solar:alt-arrow-right-linear" size={10} color="var(--text-faint)" />
          <button class="crumb-func" title="Go to symbol (⌘⇧O)" on:click={openSymbolPicker}>
            <Icon icon="solar:code-square-bold" size={11} color="var(--info)" />
            <span>{currentFunc.name}</span>
            <Icon icon="solar:alt-arrow-down-linear" size={9} color="var(--text-faint)" />
          </button>
        {:else}
          <button class="crumb-func ghost" title="Go to symbol (⌘⇧O)" on:click={openSymbolPicker}>
            <Icon icon="solar:list-bold" size={11} color="var(--text-faint)" />
            <span>Symbols</span>
          </button>
        {/if}
      </div>
      <div class="bc-actions">
        <button class="btn icon" title="Find (⌘F)" on:click={openFindPanel}>
          <Icon icon="solar:magnifer-linear" size={12} />
        </button>
        {#if dirty}
          <span class="dirty-dot" title="Unsaved changes">●</span>
          <button class="btn icon" title="Save (⌘S)" on:click={saveFile}>
            <Icon icon="solar:diskette-bold" size={12} />
          </button>
        {/if}
      </div>
    </div>
  {/if}

  {#if inlineVarText}
    <div class="inline-bar">{inlineVarText}</div>
  {/if}

  <div class="editor-wrap">
    {#if !path}
      <div class="empty">
        <Icon icon="solar:code-2-bold-duotone" size={24} color="var(--text-faint)" />
        <span>Open a file from the Files panel or press <strong>⌘O</strong></span>
      </div>
    {/if}
    <div class="cm-container" hidden={!path} bind:this={editorEl}></div>
  </div>
</div>

<SymbolPicker bind:open={symbolPickerOpen} {symbols} on:pick={(e) => gotoSymbol(e.detail)} />

<!-- Context menu -->
{#if ctxOpen}
  <div class="ctx" style:left="{ctxX}px" style:top="{ctxY}px" role="menu" tabindex="-1" on:click|stopPropagation on:keydown|stopPropagation>
    {#if !editingType}
      {#if bpLines.includes(ctxLine)}
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
  .sp-root { display:flex; flex-direction:column; height:100%; min-height:0; overflow:hidden; }

  /* Breadcrumb strip */
  .breadcrumb {
    display:flex; align-items:center; gap:var(--space-2);
    height:28px; padding:0 var(--space-2);
    border-bottom:1px solid var(--border-subtle);
    background:var(--bg-elevated);
    flex-shrink:0;
    overflow:hidden;
  }
  .crumbs {
    display:flex; align-items:center; gap:4px;
    flex:1; min-width:0;
    font-family:var(--font-mono); font-size:var(--text-xs);
    overflow:hidden;
  }
  .crumb {
    color:var(--text-faint);
    white-space:nowrap;
  }
  .crumb.leaf { color:var(--text-muted); }
  .crumb-func {
    display:inline-flex; align-items:center; gap:4px;
    background:transparent; border:0;
    color:var(--text);
    font-family:var(--font-mono); font-size:var(--text-xs);
    padding:2px 6px; border-radius:4px; cursor:pointer;
    white-space:nowrap;
  }
  .crumb-func:hover { background:var(--bg-subtle); }
  .crumb-func.ghost { color:var(--text-faint); }
  .bc-actions { display:flex; align-items:center; gap:2px; flex-shrink:0; }
  .dirty-dot { color:var(--warning); font-size:10px; }

  .editor-wrap { flex:1; overflow:hidden; display:flex; flex-direction:column; min-height:0; }
  .cm-container { flex:1; overflow:hidden; }
  .cm-container :global(.cm-editor) { height:100%; }
  .empty { display:flex; flex-direction:column; align-items:center; gap:var(--space-2); padding:var(--space-8) var(--space-4); color:var(--text-faint); font-size:var(--text-sm); text-align:center; font-family:var(--font-ui); flex:1; justify-content:center; }
  .inline-bar {
    padding:2px var(--space-3); font-family:var(--font-mono); font-size:var(--text-xs);
    color:var(--text-faint); background:var(--bg-elevated); border-bottom:1px solid var(--border-subtle);
    overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-style:italic; flex-shrink:0;
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
