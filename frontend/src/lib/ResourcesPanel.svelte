<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";
  import * as SessionService from "../../bindings/github.com/jp/DelveUI/internal/services/sessionservice";

  type Res = {
    pid: number;
    label: string;
    state: string;
    port: number;
    rssMb: number;
    cpu: string;
    elapsed: string;
  };

  let appRes: Res | null = null;
  let sessionRes: Res[] = [];
  let timer: any;

  async function refresh() {
    try {
      appRes = (await SessionService.AppResources()) as any;
      const list = (await SessionService.AllResources()) as any as Res[];
      sessionRes = list ?? [];
    } catch { /* ignore */ }
  }

  onMount(() => {
    refresh();
    timer = setInterval(refresh, 3000);
  });
  onDestroy(() => clearInterval(timer));

  function fmtMb(n: number) {
    return n < 1 ? `${(n * 1024).toFixed(0)} KB` : `${n.toFixed(1)} MB`;
  }
</script>

<PanelHeader title="Resources">
  <button class="btn icon" title="Refresh" on:click={refresh}>
    <Icon icon="solar:refresh-linear" size={13} />
  </button>
</PanelHeader>

<div class="body">
  <table>
    <thead>
      <tr>
        <th>Process</th>
        <th>PID</th>
        <th>Memory</th>
        <th>CPU</th>
        <th>Uptime</th>
      </tr>
    </thead>
    <tbody>
      {#if appRes}
        <tr class="app-row">
          <td class="label">
            <Icon icon="solar:monitor-bold" size={12} color="var(--accent)" />
            {appRes.label}
          </td>
          <td class="mono">{appRes.pid}</td>
          <td class="mono">{fmtMb(appRes.rssMb)}</td>
          <td class="mono">{appRes.cpu}</td>
          <td class="mono">{appRes.elapsed}</td>
        </tr>
      {/if}
      {#each sessionRes as s}
        <tr>
          <td class="label">
            <span class="state-dot state-{s.state}">●</span>
            {s.label}
            {#if s.port}<span class="port">:{s.port}</span>{/if}
          </td>
          <td class="mono">{s.pid || "-"}</td>
          <td class="mono">{s.pid ? fmtMb(s.rssMb) : "-"}</td>
          <td class="mono">{s.pid ? s.cpu : "-"}</td>
          <td class="mono">{s.pid ? s.elapsed : "-"}</td>
        </tr>
      {/each}
      {#if !sessionRes.length}
        <tr>
          <td colspan="5" class="empty">No debug sessions</td>
        </tr>
      {/if}
    </tbody>
  </table>
</div>

<style>
  .body { flex:1; overflow:auto; }
  table { width:100%; border-collapse:collapse; font-size:var(--text-xs); }
  th {
    text-align:left;
    padding:var(--space-1) var(--space-2);
    color:var(--text-faint);
    font-weight:600;
    text-transform:uppercase;
    letter-spacing:0.5px;
    border-bottom:1px solid var(--border-subtle);
    position:sticky;
    top:0;
    background:var(--bg);
  }
  td { padding:var(--space-1) var(--space-2); color:var(--text); }
  tr:hover td { background:var(--bg-subtle); }
  .app-row td { color:var(--accent); }
  .label { display:flex; align-items:center; gap:var(--space-1); }
  .mono { font-family:var(--font-mono); }
  .port { color:var(--text-faint); }
  .empty { color:var(--text-faint); padding:var(--space-3); text-align:center; }
</style>
