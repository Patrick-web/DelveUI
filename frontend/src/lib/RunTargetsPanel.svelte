<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Events } from "@wailsio/runtime";
  import {
    runTargets,
    targetsLoading,
    targetsLastScanned,
    refreshTargets,
    launchTarget,
    attachToProcess,
    workspace,
    sessions,
    activeSessionId,
    stopSession,
    restartSession,
    dismissSession,
  } from "./store";
  import { recency } from "./recency-store";
  import Icon from "./Icon.svelte";

  type Target = {
    id: string;
    provider: string;
    kind: string;
    label: string;
    description?: string;
    dir: string;
    program: string;
    pid?: number;
    envFiles?: string[];
    sourceFile?: string;
  };

  let filter = "";
  let pidInput = "";
  let attachOpen = false;
  let envPopoverFor = "";
  // Targets where the user has clicked Run but launchTarget() hasn't returned
  // yet. Discovery-launched sessions don't get a session:event placeholder
  // (their cfgId isn't in workspace.configs), so without this the row stays
  // idle until the backend registers the real session.
  let launching = new Set<string>();

  $: hasWorkspace = !!$workspace?.root;

  // Group targets by kind, sort within each group by recency (most
  // recently launched first), then alphabetical for never-launched.
  // Display priority: run, test, attach, then anything else.
  const KIND_ORDER = ["run", "test", "benchmark", "example", "attach"];
  $: grouped = (() => {
    const filtered: Target[] = ($runTargets ?? []).filter((t: Target) => {
      if (!filter) return true;
      const hay = (t.label + " " + (t.description ?? "")).toLowerCase();
      return hay.includes(filter.toLowerCase());
    });
    const rec = $recency;
    const buckets: Record<string, Target[]> = {};
    for (const t of filtered) (buckets[t.kind] ||= []).push(t);
    for (const k of Object.keys(buckets)) {
      buckets[k].sort((a, b) => {
        const aTs = rec[a.id] ?? 0;
        const bTs = rec[b.id] ?? 0;
        if (aTs !== bTs) return bTs - aTs;
        return a.label.localeCompare(b.label, undefined, { sensitivity: "base" });
      });
    }
    const keys = [
      ...KIND_ORDER.filter((k) => buckets[k]),
      ...Object.keys(buckets).filter((k) => !KIND_ORDER.includes(k)),
    ];
    return keys.map((k) => ({ kind: k, items: buckets[k] }));
  })();

  function kindTitle(k: string): string {
    if (k === "run") return "Run";
    if (k === "test") return "Tests";
    if (k === "benchmark") return "Benchmarks";
    if (k === "example") return "Examples";
    if (k === "attach") return "Attach to process";
    return k;
  }

  // Map target.id → live session (cfg.id is set to target.id when launching
  // via the discovery layer, so cfgId on the session matches). Re-evaluates
  // automatically when sessions or runTargets change.
  $: sessionByTarget = (() => {
    const m: Record<string, any> = {};
    for (const s of Object.values($sessions)) {
      if (s.cfgId) m[s.cfgId] = s;
    }
    return m;
  })();

  function isLiveState(s?: string) {
    return s === "running" || s === "stopped" || s === "starting";
  }

  async function onRefresh() { await refreshTargets(); }

  async function onLaunch(id: string) {
    launching.add(id);
    launching = launching;
    try {
      await launchTarget(id);
    } finally {
      launching.delete(id);
      launching = launching;
    }
  }

  async function onStop(sid: string) { await stopSession(sid); }
  async function onRestart(sid: string) { await restartSession(sid); }
  async function onDismiss(sid: string) { await dismissSession(sid); }
  function onSelect(sid: string) { activeSessionId.set(sid); }

  async function onAttachManual() {
    const pid = parseInt(pidInput.trim(), 10);
    if (!Number.isFinite(pid) || pid <= 0) return;
    await attachToProcess(pid);
    pidInput = "";
    attachOpen = false;
  }

  function envBadgeLabel(t: Target): string {
    const n = t.envFiles?.length ?? 0;
    return n === 1 ? "env: 1 file" : `env: ${n} files`;
  }

  function basename(p: string): string {
    return p.split("/").pop() ?? p;
  }

  function shorten(p: string, root?: string): string {
    if (root && p.startsWith(root + "/")) return p.slice(root.length + 1);
    return p;
  }

  function lastScannedText(d: Date | null): string {
    if (!d) return "";
    const sec = Math.max(0, Math.floor((Date.now() - d.getTime()) / 1000));
    if (sec < 5) return "just now";
    if (sec < 60) return `${sec}s ago`;
    if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
    return `${Math.floor(sec / 3600)}h ago`;
  }

  // Auto-refresh once when the panel is first mounted with a workspace open
  // and we have no targets yet. Subsequent refreshes are user-initiated, so
  // a slow `find` can't surprise users.
  let mounted = false;
  let unlistenStart: (() => void) | null = null;
  let unlistenDone: (() => void) | null = null;
  onMount(async () => {
    mounted = true;
    unlistenStart = (await Events.On("discovery:start", () => targetsLoading.set(true))) as any;
    unlistenDone = (await Events.On("discovery:done", () => targetsLoading.set(false))) as any;
    if (hasWorkspace && ($runTargets ?? []).length === 0) {
      await refreshTargets();
    }
  });
  onDestroy(() => {
    mounted = false;
    unlistenStart?.();
    unlistenDone?.();
  });

  // Trigger a re-scan when the workspace changes (open another project).
  let lastRoot = "";
  $: if (mounted && hasWorkspace && $workspace?.root && $workspace.root !== lastRoot) {
    lastRoot = $workspace.root;
    refreshTargets().catch(() => {});
  }

  function toggleEnvPopover(id: string) {
    envPopoverFor = envPopoverFor === id ? "" : id;
  }
</script>

<div class="rt-root">
  <div class="rt-toolbar">
    <div class="rt-filter">
      <Icon icon="solar:magnifer-linear" size={12} color="var(--text-faint)" />
      <input
        class="rt-filter-input"
        placeholder="Filter targets…"
        bind:value={filter}
        spellcheck="false"
      />
      {#if filter}
        <button class="rt-clear" title="Clear" on:click={() => (filter = "")}>
          <Icon icon="solar:close-circle-linear" size={12} />
        </button>
      {/if}
    </div>
    <button
      class="rt-icon-btn"
      title="Attach to process by PID"
      on:click={() => (attachOpen = !attachOpen)}
    >
      <Icon icon="solar:link-bold" size={12} />
    </button>
    <button
      class="rt-icon-btn"
      class:spinning={$targetsLoading}
      title={$targetsLoading ? "Scanning…" : "Refresh targets"}
      disabled={$targetsLoading}
      on:click={onRefresh}
    >
      <Icon icon="solar:refresh-linear" size={12} />
    </button>
  </div>

  {#if attachOpen}
    <div class="rt-attach">
      <input
        class="rt-attach-input"
        placeholder="Process ID"
        bind:value={pidInput}
        on:keydown={(e) => { if (e.key === "Enter") onAttachManual(); }}
        spellcheck="false"
      />
      <button class="rt-attach-go" on:click={onAttachManual}>Attach</button>
    </div>
  {/if}

  <div class="rt-body">
    {#if !hasWorkspace}
      <div class="empty">
        Open a workspace to discover run targets.
      </div>
    {:else if $targetsLoading && ($runTargets ?? []).length === 0}
      <div class="empty">
        <span class="spinner"></span> Scanning workspace…
      </div>
    {:else if grouped.length === 0}
      <div class="empty">
        {filter
          ? `No matches for "${filter}".`
          : "No runnable targets discovered. Click refresh to retry."}
      </div>
    {:else}
      {#each grouped as group}
        <div class="rt-group">
          <div class="rt-group-head">
            <span>{kindTitle(group.kind)}</span>
            <span class="rt-group-count">{group.items.length}</span>
          </div>
          {#each group.items as t (t.id)}
            {@const sess = sessionByTarget[t.id]}
            {@const pending = launching.has(t.id)}
            {@const live = isLiveState(sess?.state)}
            {@const state = pending ? "starting" : sess?.state ?? "idle"}
            <!-- svelte-ignore a11y-click-events-have-key-events -->
            <!-- svelte-ignore a11y-no-static-element-interactions -->
            <div
              class="rt-row"
              class:active={sess && sess.id === $activeSessionId}
              title={t.description ?? ""}
              role={sess ? "button" : undefined}
              tabindex={sess ? 0 : -1}
              on:click={() => sess && onSelect(sess.id)}
            >
              <span class="rt-kind rt-kind-{t.kind} rt-state-{state}"></span>
              <span class="rt-label">
                <span class="rt-label-main">{t.label}</span>
                {#if t.description}
                  <span class="rt-label-sub">{t.description}</span>
                {/if}
              </span>
              {#if t.envFiles && t.envFiles.length > 0}
                <button
                  class="rt-env-badge"
                  title={t.envFiles.join("\n")}
                  on:click|stopPropagation={() => toggleEnvPopover(t.id)}
                >
                  {envBadgeLabel(t)}
                </button>
              {/if}
              <span class="rt-actions">
                {#if state === "starting"}
                  <span class="rt-spinner" title="Starting…" aria-label="Starting"></span>
                {:else if live}
                  <button
                    class="rt-act"
                    title="Restart"
                    on:click|stopPropagation={() => onRestart(sess.id)}
                  >
                    <Icon icon="solar:restart-bold" size={12} />
                  </button>
                  <button
                    class="rt-act rt-act-stop"
                    title="Stop"
                    on:click|stopPropagation={() => onStop(sess.id)}
                  >
                    <Icon icon="solar:stop-bold" size={12} />
                  </button>
                {:else}
                  <button
                    class="rt-act rt-act-play"
                    title={t.kind === "attach" ? "Attach" : "Run"}
                    on:click|stopPropagation={() => onLaunch(t.id)}
                  >
                    <Icon icon="solar:play-bold" size={12} />
                  </button>
                  {#if sess}
                    <button
                      class="rt-act rt-act-dismiss"
                      title={state === "error" ? "Dismiss failed session" : "Dismiss"}
                      on:click|stopPropagation={() => onDismiss(sess.id)}
                    >
                      <Icon icon="solar:close-circle-linear" size={12} />
                    </button>
                  {/if}
                {/if}
              </span>
            </div>
            {#if envPopoverFor === t.id && t.envFiles}
              <div class="rt-env-popover">
                <div class="rt-env-popover-head">
                  Env files (outer → inner, last wins)
                </div>
                {#each t.envFiles as f}
                  <div class="rt-env-popover-row" title={f}>
                    <Icon icon="solar:document-text-linear" size={11} />
                    <span class="rt-env-popover-name">{basename(f)}</span>
                    <span class="rt-env-popover-path">{shorten(f, $workspace?.root)}</span>
                  </div>
                {/each}
              </div>
            {/if}
          {/each}
        </div>
      {/each}
    {/if}
  </div>

  {#if $targetsLastScanned}
    <div class="rt-footer">
      Last scanned {lastScannedText($targetsLastScanned)}
    </div>
  {/if}
</div>

<style>
  .rt-root {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }

  .rt-toolbar {
    display: flex;
    align-items: center;
    gap: 4px;
    margin: 6px 6px 4px 6px;
    flex-shrink: 0;
  }
  .rt-filter {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 8px;
    height: 26px;
    background: var(--bg);
    border: 1px solid var(--border-subtle);
    border-radius: 5px;
  }
  .rt-filter:focus-within { border-color: var(--accent); }
  .rt-filter-input {
    flex: 1;
    background: transparent;
    border: 0;
    color: var(--text);
    font-size: var(--text-xs);
    font-family: var(--font-ui);
    outline: none;
    padding: 0;
  }
  .rt-clear, .rt-icon-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid transparent;
    color: var(--text-faint);
    cursor: pointer;
    padding: 0;
    width: 26px;
    height: 26px;
    border-radius: 4px;
  }
  .rt-icon-btn { border-color: var(--border-subtle); }
  .rt-icon-btn:hover { color: var(--text); background: rgba(255,255,255,0.04); }
  .rt-icon-btn:disabled { opacity: 0.5; cursor: default; }
  .rt-icon-btn.spinning { animation: rt-spin 1s linear infinite; }

  .rt-attach {
    display: flex;
    gap: 4px;
    margin: 0 6px 6px 6px;
    flex-shrink: 0;
  }
  .rt-attach-input {
    flex: 1;
    background: var(--bg);
    border: 1px solid var(--border-subtle);
    border-radius: 4px;
    color: var(--text);
    padding: 0 8px;
    height: 26px;
    font-size: var(--text-xs);
    font-family: var(--font-mono);
    outline: none;
  }
  .rt-attach-input:focus { border-color: var(--accent); }
  .rt-attach-go {
    background: var(--accent);
    color: #fff;
    border: 0;
    height: 26px;
    padding: 0 12px;
    border-radius: 4px;
    cursor: pointer;
    font-size: var(--text-xs);
    font-weight: 600;
  }

  .rt-body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 0 0 4px 0;
  }
  .empty {
    padding: var(--space-3);
    color: var(--text-faint);
    font-size: var(--text-sm);
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .rt-group { display: flex; flex-direction: column; }
  .rt-group-head {
    display: flex;
    align-items: center;
    gap: 6px;
    height: 22px;
    padding: 0 12px;
    color: var(--text-faint);
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.8px;
    text-transform: uppercase;
  }
  .rt-group-count {
    background: var(--bg-elevated);
    color: var(--text-muted);
    padding: 1px 5px;
    border-radius: 7px;
    font-size: 9px;
    letter-spacing: 0;
    text-transform: none;
  }

  .rt-row {
    display: flex;
    align-items: center;
    gap: 6px;
    height: 30px;
    margin: 0 4px;
    padding: 0 6px 0 8px;
    border-radius: 5px;
    cursor: default;
    font-size: var(--text-sm);
    color: var(--text);
    transition: background 80ms ease;
  }
  .rt-row[role="button"] { cursor: pointer; }
  .rt-row:hover { background: rgba(255, 255, 255, 0.04); }
  .rt-row.active { background: var(--accent); color: #fff; }
  .rt-row.active .rt-label-sub { color: rgba(255,255,255,0.65); }

  .rt-kind {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .rt-kind-run { background: #4cc38a; }
  .rt-kind-test { background: #f5a623; }
  .rt-kind-attach { background: #b083ee; }
  .rt-kind-benchmark { background: #5ec1e4; }
  .rt-kind-example { background: #e4906c; }
  /* Live-state pulse: takes precedence over kind colour while a session is up. */
  .rt-state-running { background: var(--success) !important; box-shadow: 0 0 6px var(--success); }
  .rt-state-stopped { background: var(--warning, #f5a623) !important; box-shadow: 0 0 6px rgba(245,166,35,0.6); }
  .rt-state-starting { background: var(--text-faint) !important; animation: rt-blink 1s linear infinite; }
  @keyframes rt-blink { 50% { opacity: 0.3; } }

  .rt-actions { display: inline-flex; gap: 2px; flex-shrink: 0; }
  .rt-act-stop { color: var(--danger); }
  .rt-act-dismiss { color: var(--text-faint); }
  .rt-act-dismiss:hover { color: var(--danger); }
  .rt-row.active .rt-act-play { color: #a5e0a0; }
  .rt-row.active .rt-act-stop { color: #ffb3b3; }
  .rt-row.active .rt-act-dismiss { color: rgba(255,255,255,0.55); }
  .rt-row.active .rt-act-dismiss:hover { color: #ffb3b3; }
  .rt-spinner {
    display: inline-block;
    width: 13px; height: 13px;
    margin: 0 4px;
    border: 1.5px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: rt-spin 0.8s linear infinite;
  }

  .rt-label {
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
  }
  .rt-label-main {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    line-height: 1.1;
  }
  .rt-label-sub {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-faint);
    font-family: var(--font-mono);
    font-size: 10px;
    line-height: 1.1;
  }

  .rt-env-badge {
    background: rgba(255,255,255,0.05);
    border: 1px solid var(--border-subtle);
    border-radius: 3px;
    color: var(--text-faint);
    cursor: pointer;
    font-family: var(--font-mono);
    font-size: 9px;
    padding: 2px 6px;
    flex-shrink: 0;
  }
  .rt-env-badge:hover { color: var(--text); border-color: var(--accent); }

  .rt-act {
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
    flex-shrink: 0;
  }
  .rt-row:hover .rt-act { opacity: 1; }
  .rt-act:hover { background: rgba(255,255,255,0.1); }
  .rt-act-play { color: var(--success); }

  .rt-env-popover {
    margin: 2px 8px 6px 24px;
    padding: 6px 8px;
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 4px;
    font-size: var(--text-xs);
  }
  .rt-env-popover-head {
    color: var(--text-faint);
    font-size: 9px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 4px;
  }
  .rt-env-popover-row {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 2px 0;
    color: var(--text-muted);
  }
  .rt-env-popover-name {
    font-family: var(--font-mono);
    color: var(--text);
  }
  .rt-env-popover-path {
    margin-left: auto;
    color: var(--text-faint);
    font-family: var(--font-mono);
    font-size: 10px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .rt-footer {
    flex-shrink: 0;
    padding: 4px 12px;
    border-top: 1px solid var(--border-subtle);
    color: var(--text-faint);
    font-size: 10px;
  }

  .spinner {
    display: inline-block;
    width: 12px;
    height: 12px;
    border: 1.5px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: rt-spin 0.8s linear infinite;
  }
  @keyframes rt-spin { to { transform: rotate(360deg); } }
</style>
