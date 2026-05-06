<script lang="ts">
  import { onMount, tick } from "svelte";
  import Icon from "./Icon.svelte";
  import { manualSourcePath, scrollToLineRequest } from "./store";
  import { setActivePanel } from "./panels/layout";
  import {
    searchState,
    setSearchOptions,
    runSearch,
    cancelSearch,
    toggleFileCollapsed,
    groupByFile,
    type SearchMatch,
  } from "./search-store";

  // Imperative focus handle exposed so the sidebar / shortcut can land here.
  let inputEl: HTMLInputElement | null = null;
  export function focusInput() {
    inputEl?.focus();
    inputEl?.select();
  }

  let showFilters = false;

  // Debounce typing → search. 220ms feels responsive without hammering the
  // backend on every keystroke.
  let debounceHandle: any = null;
  function scheduleSearch() {
    if (debounceHandle) clearTimeout(debounceHandle);
    debounceHandle = setTimeout(() => {
      debounceHandle = null;
      runSearch();
    }, 220);
  }

  function onQueryInput(e: Event) {
    const v = (e.currentTarget as HTMLInputElement).value;
    setSearchOptions({ query: v });
    // New query → forget the user's previous "show all" so big result sets
    // don't accidentally render unbounded after they refine.
    liftCap = false;
    scheduleSearch();
  }

  function onToggleRegex() {
    setSearchOptions({ regex: !$searchState.options.regex });
    scheduleSearch();
  }
  function onToggleCase() {
    setSearchOptions({ caseSensitive: !$searchState.options.caseSensitive });
    scheduleSearch();
  }
  function onToggleWholeWord() {
    setSearchOptions({ wholeWord: !$searchState.options.wholeWord });
    scheduleSearch();
  }
  function onIncludesInput(e: Event) {
    setSearchOptions({ includes: (e.currentTarget as HTMLInputElement).value });
    scheduleSearch();
  }
  function onExcludesInput(e: Event) {
    setSearchOptions({ excludes: (e.currentTarget as HTMLInputElement).value });
    scheduleSearch();
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Enter") {
      if (debounceHandle) { clearTimeout(debounceHandle); debounceHandle = null; }
      runSearch();
    } else if (e.key === "Escape") {
      cancelSearch();
    }
  }

  function openMatch(m: SearchMatch) {
    manualSourcePath.set(m.path);
    setActivePanel("right", "source");
    // Two-step set lets SourcePanel react in two phases: file load, then scroll.
    tick().then(() => scrollToLineRequest.set(m.line));
  }

  // Render line text with the matched ranges wrapped in <mark>. Plain string
  // splice — Ranges are byte offsets, but for ASCII source files they line up
  // with character indices in JS, and the typical search hit is ASCII.
  function renderMatchHTML(text: string, ranges: [number, number][]): string {
    if (!ranges?.length) return escapeHTML(text);
    let out = "";
    let cursor = 0;
    for (const [s, e] of ranges) {
      if (s < cursor) continue;
      out += escapeHTML(text.slice(cursor, s));
      out += `<mark>${escapeHTML(text.slice(s, e))}</mark>`;
      cursor = e;
    }
    out += escapeHTML(text.slice(cursor));
    return out;
  }
  function escapeHTML(s: string): string {
    return s
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  // Render caps. The store keeps every match the backend streamed; we just
  // hide the long tail so Svelte doesn't drop a few thousand DOM rows in one
  // tick. The user can lift the cap from the banner if they really want to
  // see everything.
  const TOTAL_RENDER_CAP = 500;
  const PER_FILE_CAP = 50;
  let liftCap = false;

  $: allGroups = groupByFile($searchState.results);
  $: visibleGroups = (() => {
    if (liftCap) return allGroups;
    const out: typeof allGroups = [];
    let used = 0;
    for (const g of allGroups) {
      if (used >= TOTAL_RENDER_CAP) break;
      const remaining = TOTAL_RENDER_CAP - used;
      const sliceLen = Math.min(g.matches.length, PER_FILE_CAP, remaining);
      out.push({ ...g, matches: g.matches.slice(0, sliceLen) });
      used += sliceLen;
    }
    return out;
  })();
  $: hiddenCount = $searchState.results.length - visibleGroups.reduce((n, g) => n + g.matches.length, 0);

  function fileName(rel: string): string {
    const i = rel.lastIndexOf("/");
    return i >= 0 ? rel.slice(i + 1) : rel;
  }
  function fileDir(rel: string): string {
    const i = rel.lastIndexOf("/");
    return i >= 0 ? rel.slice(0, i) : "";
  }

  onMount(() => {
    // If the user lands here with a previously typed query, focus and select.
    requestAnimationFrame(() => focusInput());
  });
</script>

<div class="sp-root">
  <div class="sp-input-row">
    <Icon icon="solar:magnifer-linear" size={12} color="var(--text-faint)" />
    <input
      bind:this={inputEl}
      type="text"
      spellcheck="false"
      placeholder="Search workspace…"
      value={$searchState.options.query}
      on:input={onQueryInput}
      on:keydown={onKey}
    />
    <button
      class="sp-icon"
      class:on={$searchState.options.caseSensitive}
      title="Match case"
      on:click={onToggleCase}
    >
      <span class="sp-icon-text">Aa</span>
    </button>
    <button
      class="sp-icon"
      class:on={$searchState.options.wholeWord}
      title="Match whole word"
      on:click={onToggleWholeWord}
    >
      <span class="sp-icon-text sp-ww">ab</span>
    </button>
    <button
      class="sp-icon"
      class:on={$searchState.options.regex}
      title="Use regular expression"
      on:click={onToggleRegex}
    >
      <span class="sp-icon-text">.*</span>
    </button>
    <button
      class="sp-icon"
      class:on={showFilters}
      title="Toggle file filters"
      on:click={() => (showFilters = !showFilters)}
    >
      <Icon icon="solar:filter-linear" size={12} />
    </button>
  </div>

  {#if showFilters}
    <div class="sp-filters">
      <label>
        <span>Include</span>
        <input
          type="text"
          spellcheck="false"
          placeholder="*.go, internal/**"
          value={$searchState.options.includes}
          on:input={onIncludesInput}
          on:keydown={onKey}
        />
      </label>
      <label>
        <span>Exclude</span>
        <input
          type="text"
          spellcheck="false"
          placeholder="**/testdata/**, *.pb.go"
          value={$searchState.options.excludes}
          on:input={onExcludesInput}
          on:keydown={onKey}
        />
      </label>
    </div>
  {/if}

  <div class="sp-status">
    {#if $searchState.status === "searching"}
      <span class="muted">Searching… ({$searchState.results.length} matches so far)</span>
      <button class="link" on:click={cancelSearch}>Cancel</button>
    {:else if $searchState.status === "error"}
      <span class="err">Error: {$searchState.errorMessage}</span>
    {:else if $searchState.status === "done" && $searchState.summary}
      <span class="muted">
        {$searchState.summary.matches} match{$searchState.summary.matches === 1 ? "" : "es"}
        in {$searchState.summary.files} file{$searchState.summary.files === 1 ? "" : "s"}
        ({$searchState.summary.durationMs}ms){$searchState.summary.truncated ? " · truncated" : ""}{$searchState.summary.cancelled ? " · cancelled" : ""}
      </span>
    {:else if $searchState.options.query.trim() && $searchState.status === "idle"}
      <span class="muted">Press Enter to search</span>
    {/if}
  </div>

  <div class="sp-results">
    {#each visibleGroups as g (g.path)}
      {@const collapsed = !!$searchState.collapsedFiles[g.path]}
      {@const fileGroup = allGroups.find((x) => x.path === g.path)}
      {@const fullCount = fileGroup ? fileGroup.matches.length : g.matches.length}
      <div class="sp-file">
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
        <div class="sp-file-head" role="button" tabindex="0" on:click={() => toggleFileCollapsed(g.path)}>
          <Icon icon={collapsed ? "solar:alt-arrow-right-linear" : "solar:alt-arrow-down-linear"} size={10} />
          <span class="sp-file-name">{fileName(g.rel)}</span>
          <span class="sp-file-dir">{fileDir(g.rel)}</span>
          <span class="sp-file-count">{fullCount}</span>
        </div>
        {#if !collapsed}
          {#each g.matches as m (m.line + ":" + m.col + ":" + m.text)}
            <button class="sp-match" on:click={() => openMatch(m)}>
              <span class="sp-line-no">{m.line}</span>
              <span class="sp-line-text">{@html renderMatchHTML(m.text, m.ranges)}</span>
            </button>
          {/each}
          {#if fullCount > g.matches.length}
            <div class="sp-more-row">+{fullCount - g.matches.length} more in this file</div>
          {/if}
        {/if}
      </div>
    {/each}

    {#if hiddenCount > 0 && !liftCap}
      <button class="sp-lift" on:click={() => (liftCap = true)}>
        Showing {$searchState.results.length - hiddenCount} of {$searchState.results.length} matches —
        <span class="sp-lift-cta">show all</span>
      </button>
    {/if}

    {#if $searchState.status === "done" && allGroups.length === 0 && $searchState.options.query.trim()}
      <div class="sp-empty">No matches.</div>
    {/if}
  </div>
</div>

<style>
  .sp-root {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }
  .sp-input-row {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 10px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .sp-input-row input {
    flex: 1;
    min-width: 0;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    padding: 4px 8px;
    outline: none;
  }
  .sp-input-row input:focus { border-color: var(--accent); }
  .sp-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    cursor: pointer;
  }
  .sp-icon:hover { background: var(--bg-elevated); color: var(--text); }
  .sp-icon.on {
    background: var(--accent-subtle);
    border-color: var(--accent);
    color: var(--text);
  }
  .sp-icon-text {
    font-family: var(--font-mono);
    font-size: 10px;
    line-height: 1;
  }
  /* Whole-word glyph: underline-bordered "ab" reads as "match exact word". */
  .sp-ww {
    border-bottom: 1px solid currentColor;
    padding-bottom: 1px;
  }

  .sp-filters {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 6px 10px 8px;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-subtle);
  }
  .sp-filters label {
    display: grid;
    grid-template-columns: 56px 1fr;
    align-items: center;
    gap: 6px;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .sp-filters input {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    padding: 3px 6px;
    outline: none;
  }
  .sp-filters input:focus { border-color: var(--accent); }

  .sp-status {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 10px;
    font-size: var(--text-xs);
    color: var(--text-faint);
    min-height: 18px;
  }
  .sp-status .muted { color: var(--text-faint); }
  .sp-status .err { color: var(--danger); }
  .sp-status .link {
    background: transparent;
    border: 0;
    color: var(--accent);
    cursor: pointer;
    font-size: var(--text-xs);
    padding: 0;
  }

  .sp-results {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }

  .sp-file { display: flex; flex-direction: column; }
  .sp-file-head {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 3px 10px;
    cursor: pointer;
    font-size: var(--text-xs);
    user-select: none;
    background: var(--bg-subtle);
    border-bottom: 1px solid var(--border-subtle);
    border-top: 1px solid var(--border-subtle);
  }
  .sp-file-head:hover { background: var(--bg-elevated); }
  .sp-file-name {
    color: var(--text);
    font-weight: 600;
  }
  .sp-file-dir {
    color: var(--text-faint);
    font-family: var(--font-mono);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0;
  }
  .sp-file-count {
    color: var(--text-muted);
    background: var(--bg);
    border: 1px solid var(--border-subtle);
    border-radius: 9px;
    padding: 0 6px;
    font-size: 10px;
    line-height: 14px;
  }

  .sp-match {
    display: grid;
    grid-template-columns: 36px 1fr;
    align-items: baseline;
    gap: 6px;
    width: 100%;
    background: transparent;
    border: 0;
    border-bottom: 1px solid var(--border-subtle);
    padding: 3px 10px;
    text-align: left;
    color: var(--text-muted);
    cursor: pointer;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    line-height: 1.5;
  }
  .sp-match:hover { background: var(--bg-elevated); color: var(--text); }
  .sp-line-no {
    color: var(--text-faint);
    text-align: right;
  }
  .sp-line-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sp-line-text :global(mark) {
    background: rgba(255, 204, 0, 0.32);
    color: var(--text);
    border-radius: 2px;
    padding: 0 1px;
  }

  .sp-empty {
    padding: 16px 10px;
    color: var(--text-faint);
    font-size: var(--text-sm);
    text-align: center;
  }

  .sp-more-row {
    padding: 3px 10px 3px 36px;
    color: var(--text-faint);
    font-size: var(--text-xs);
    font-style: italic;
    border-bottom: 1px solid var(--border-subtle);
  }
  .sp-lift {
    width: 100%;
    background: transparent;
    border: 0;
    border-top: 1px solid var(--border-subtle);
    padding: 8px 10px;
    color: var(--text-faint);
    font-size: var(--text-xs);
    cursor: pointer;
    text-align: center;
  }
  .sp-lift:hover { background: var(--bg-elevated); color: var(--text); }
  .sp-lift-cta { color: var(--accent); }
</style>
