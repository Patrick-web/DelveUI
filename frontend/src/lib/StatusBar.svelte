<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import {
    workspace,
    sessions,
    sessionState,
    activeSession,
    activeSessionId,
  } from "./store";
  import * as SessionService from "../../bindings/github.com/jp/DelveUI/internal/services/sessionservice";

  let appMem = "";
  let uptime = "";
  let timer: any;
  const startTime = Date.now();

  $: running = Object.values($sessions).filter(
    (s) => s.state === "running" || s.state === "stopped",
  ).length;

  $: state = $activeSession?.state ?? "idle";

  $: stopped = (() => {
    if (!$activeSessionId) return null;
    const st = $sessionState[$activeSessionId];
    if (!st) return null;
    const f = st.stack?.[0];
    if (!f || $activeSession?.state !== "stopped") return null;
    const p = f.source?.path ?? "";
    const short = p.split("/").slice(-2).join("/");
    return `${short}:${f.line}`;
  })();

  $: bpCount = Object.values($sessionState).reduce((n, s) => {
    for (const lines of Object.values(s.breakpoints ?? {}))
      n += (lines as number[]).length;
    return n;
  }, 0);

  $: wsName = (() => {
    const p = $workspace?.root ?? $workspace?.debugFile ?? "";
    if (!p) return "";
    return p.split("/").filter(Boolean).pop() ?? "";
  })();

  function fmtUptime() {
    const s = Math.floor((Date.now() - startTime) / 1000);
    if (s < 60) return `${s}s`;
    const m = Math.floor(s / 60);
    if (m < 60) return `${m}m`;
    const h = Math.floor(m / 60);
    return `${h}h ${m % 60}m`;
  }

  async function refreshStats() {
    uptime = fmtUptime();
    try {
      const r = (await SessionService.AppResources()) as any;
      if (r?.rssMb != null) {
        appMem = r.rssMb < 1 ? `${(r.rssMb * 1024).toFixed(0)} KB` : `${r.rssMb.toFixed(0)} MB`;
      }
    } catch { /* ignore */ }
  }

  onMount(() => {
    refreshStats();
    timer = setInterval(refreshStats, 5000);
  });
  onDestroy(() => clearInterval(timer));
</script>

<footer class="statusbar">
  <div class="left">
    {#if $activeSession}
      <span class="state-dot state-{state}">●</span>
      <span class="label">{$activeSession.label}</span>
      {#if $activeSession.port}
        <span class="dim">:{$activeSession.port}</span>
      {/if}
      {#if $activeSession.pid}
        <span class="badge">PID {$activeSession.pid}</span>
      {/if}
    {:else}
      <span class="dim">No active session</span>
    {/if}
  </div>
  <div class="mid">
    {#if stopped}<span class="stopped">{stopped}</span>{/if}
  </div>
  <div class="right">
    {#if wsName}<span>{wsName}</span><span class="sep">·</span>{/if}
    <span>{bpCount} bp</span>
    <span class="sep">·</span>
    <span>{running} running</span>
    {#if appMem}
      <span class="sep">·</span>
      <span>{appMem}</span>
    {/if}
    {#if uptime}
      <span class="sep">·</span>
      <span>{uptime}</span>
    {/if}
  </div>
</footer>

<style>
  .statusbar {
    display: flex;
    align-items: center;
    height: 22px;
    padding: 0 var(--space-3);
    background: var(--bg-elevated);
    border-top: 1px solid var(--border);
    font-size: var(--text-xs);
    color: var(--text-muted);
    flex-shrink: 0;
    gap: var(--space-3);
  }
  .left, .mid, .right {
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }
  .mid { flex: 1; justify-content: center; font-family: var(--font-mono); }
  .label { color: var(--text); }
  .stopped { color: var(--warning); }
  .dim { color: var(--text-faint); }
  .sep { color: var(--text-faint); }
  .badge {
    font-family: var(--font-mono);
    font-size: 9px;
    padding: 0 4px;
    background: var(--bg-subtle);
    border: 1px solid var(--border-subtle);
    border-radius: 3px;
    color: var(--text-faint);
  }
</style>
