<script lang="ts">
  import {
    workspace,
    sessions,
    activeSessionId,
    startSession,
    stopSession,
    restartSession,
  } from "./store";
  import Icon from "./Icon.svelte";

  // Merge configs with their running sessions, plus orphan sessions
  // (sessions whose config was removed from the workspace).
  $: rows = (() => {
    const cfgs = $workspace?.configs ?? [];
    const sessionList = Object.values($sessions);
    const byCfg: Record<string, typeof sessionList[number] | undefined> = {};
    for (const s of sessionList) byCfg[s.cfgId] = s;

    type Row = {
      key: string;
      cfgId?: string;
      sessionId?: string;
      label: string;
      mode?: string;
      state: string; // "idle" when no session; else session.state
      disabled?: boolean;
      disabledNote?: string;
    };

    const list: Row[] = [];
    for (const cfg of cfgs) {
      const s = byCfg[cfg.id];
      list.push({
        key: "cfg:" + cfg.id,
        cfgId: cfg.id,
        sessionId: s?.id,
        label: cfg.label,
        mode: cfg.mode,
        state: s?.state ?? "idle",
        disabled: cfg.disabled,
        disabledNote: cfg.disabledNote,
      });
    }
    // Orphan sessions (cfg removed)
    for (const s of sessionList) {
      if (!cfgs.some((c) => c.id === s.cfgId)) {
        list.push({
          key: "orphan:" + s.id,
          sessionId: s.id,
          label: s.label,
          state: s.state,
        });
      }
    }
    return list;
  })();

  function isRunning(state: string) {
    return state === "running" || state === "stopped" || state === "starting";
  }

  async function onPlay(cfgId: string | undefined) {
    if (!cfgId) return;
    await startSession(cfgId);
  }

  async function onStop(sessionId: string | undefined) {
    if (!sessionId) return;
    await stopSession(sessionId);
  }

  async function onRestart(sessionId: string | undefined) {
    if (!sessionId) return;
    await restartSession(sessionId);
  }

  function onSelect(sessionId: string | undefined) {
    if (sessionId) activeSessionId.set(sessionId);
  }

  // --- filter ---
  let filter = "";
  function fuzzy(q: string, s: string): boolean {
    if (!q) return true;
    const hay = s.toLowerCase();
    const needle = q.toLowerCase();
    let i = 0;
    for (const c of hay) { if (c === needle[i]) i++; if (i === needle.length) return true; }
    return false;
  }
  $: visibleRows = filter ? rows.filter((r) => fuzzy(filter, r.label)) : rows;
</script>

<div class="sp-root">
  <div class="sp-filter">
    <Icon icon="solar:magnifer-linear" size={12} color="var(--text-faint)" />
    <input
      class="sp-filter-input"
      placeholder="Filter…"
      bind:value={filter}
      spellcheck="false"
    />
    {#if filter}
      <button class="sp-clear" title="Clear" on:click={() => (filter = "")}>
        <Icon icon="solar:close-circle-linear" size={12} />
      </button>
    {/if}
  </div>
<div class="sp-body">
  {#if rows.length === 0}
    <div class="empty">
      No debug configurations.<br />
      Open a <code>debug.json</code> to add some.
    </div>
  {:else if visibleRows.length === 0}
    <div class="empty">No matches for &ldquo;{filter}&rdquo;.</div>
  {:else}
    {#each visibleRows as r (r.key)}
      {@const running = isRunning(r.state)}
      <div
        class="sp-row"
        class:active={r.sessionId && r.sessionId === $activeSessionId}
        class:disabled={r.disabled}
        role="button"
        tabindex="0"
        title={r.disabled ? r.disabledNote : r.label}
        on:click={() => onSelect(r.sessionId)}
        on:keydown={(e) => { if (e.key === "Enter") onSelect(r.sessionId); }}
      >
        <span class="state-dot state-{r.state}">●</span>
        <span class="label">{r.label}</span>
        {#if r.mode && !running}
          <span class="mode">{r.mode}</span>
        {/if}
        <span class="actions">
          {#if running}
            <button
              class="act"
              title="Restart"
              on:click|stopPropagation={() => onRestart(r.sessionId)}
            >
              <Icon icon="solar:restart-bold" size={12} />
            </button>
            <button
              class="act stop"
              title="Stop"
              on:click|stopPropagation={() => onStop(r.sessionId)}
            >
              <Icon icon="solar:stop-bold" size={12} />
            </button>
          {:else}
            <button
              class="act play"
              title={r.disabled ? r.disabledNote : "Run"}
              disabled={r.disabled}
              on:click|stopPropagation={() => onPlay(r.cfgId)}
            >
              <Icon icon="solar:play-bold" size={12} />
            </button>
          {/if}
        </span>
      </div>
    {/each}
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
  .sp-filter {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 6px 6px 4px 6px;
    padding: 0 8px;
    height: 26px;
    background: var(--bg);
    border: 1px solid var(--border-subtle);
    border-radius: 5px;
    flex-shrink: 0;
  }
  .sp-filter:focus-within { border-color: var(--accent); }
  .sp-filter-input {
    flex: 1;
    background: transparent;
    border: 0;
    color: var(--text);
    font-size: var(--text-xs);
    font-family: var(--font-ui);
    outline: none;
    padding: 0;
  }
  .sp-clear {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 0;
    color: var(--text-faint);
    cursor: pointer;
    padding: 0;
    width: 16px;
    height: 16px;
  }
  .sp-clear:hover { color: var(--text); }
  .sp-body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    display: flex;
    flex-direction: column;
    padding: 2px 0 4px 0;
  }
  .empty {
    padding: var(--space-3);
    color: var(--text-faint);
    font-size: var(--text-sm);
  }
  .empty code {
    font-family: var(--font-mono);
    background: var(--bg-elevated);
    padding: 1px 5px;
    border-radius: 3px;
  }

  .sp-row {
    display: flex;
    align-items: center;
    gap: 6px;
    height: 28px;
    margin: 0 4px;
    padding: 0 6px 0 8px;
    border-radius: 5px;
    cursor: pointer;
    font-size: var(--text-sm);
    color: var(--text);
    transition: background 80ms ease;
  }
  .sp-row:hover { background: rgba(255, 255, 255, 0.04); }
  .sp-row.active {
    background: var(--accent);
    color: #ffffff;
  }
  .sp-row.active .mode { color: rgba(255, 255, 255, 0.65); }
  .sp-row.active .state-dot { filter: brightness(1.3); }
  .sp-row.disabled { opacity: 0.5; cursor: default; }

  .label {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .mode {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-faint);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .actions {
    display: inline-flex;
    gap: 2px;
    flex-shrink: 0;
  }
  .act {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    background: transparent;
    border: 0;
    color: inherit;
    border-radius: 4px;
    cursor: pointer;
    opacity: 0.8;
  }
  .sp-row:hover .act { opacity: 1; }
  .act:hover { background: rgba(255, 255, 255, 0.1); }
  .act:disabled { opacity: 0.3; cursor: default; }
  .act.play { color: var(--success); }
  .act.stop { color: var(--danger); }
  .sp-row.active .act.play { color: #a5e0a0; }
  .sp-row.active .act.stop { color: #ffb3b3; }
</style>
