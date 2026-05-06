<script lang="ts">
  import Icon from "./Icon.svelte";
  import {
    notifications,
    dismissNotification,
    clearNotifications,
  } from "./toast";

  export let onClose: () => void = () => {};

  function fmtTime(ts: number): string {
    const sec = Math.max(0, Math.floor((Date.now() - ts) / 1000));
    if (sec < 5) return "just now";
    if (sec < 60) return `${sec}s ago`;
    if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
    if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
    return new Date(ts).toLocaleString();
  }

  function iconFor(kind: string): string {
    if (kind === "error") return "solar:shield-warning-bold";
    if (kind === "warning") return "solar:danger-triangle-bold";
    return "solar:info-circle-bold";
  }

  function colorFor(kind: string): string {
    if (kind === "error") return "var(--danger)";
    if (kind === "warning") return "var(--warning)";
    return "var(--info)";
  }

  function runAction(n: any) {
    try {
      n.action?.run();
    } catch {}
    onClose();
  }
</script>

<div class="popover" role="dialog" aria-label="Notifications">
  <div class="head">
    <span class="title">Notifications</span>
    <span class="count">{$notifications.length}</span>
    <button
      class="hd-btn"
      title="Clear all"
      on:click={clearNotifications}
      disabled={$notifications.length === 0}
    >
      Clear all
    </button>
    <button class="hd-btn icon" title="Close" on:click={onClose}>
      <Icon icon="solar:close-circle-linear" size={13} />
    </button>
  </div>

  <div class="body">
    {#if $notifications.length === 0}
      <div class="empty">No notifications yet.</div>
    {:else}
      {#each $notifications as n (n.id)}
        <div class="row n-{n.kind}">
          <Icon icon={iconFor(n.kind)} size={14} color={colorFor(n.kind)} />
          <div class="content">
            <div class="title-row">
              <span class="t-title">{n.title}</span>
              <span class="t-time">{fmtTime(n.ts)}</span>
            </div>
            {#if n.body}
              <div class="t-body">{n.body}</div>
            {/if}
            {#if n.action}
              <button class="action" on:click={() => runAction(n)}>
                {n.action.label}
              </button>
            {/if}
          </div>
          <button
            class="dismiss"
            title="Remove"
            on:click={() => dismissNotification(n.id)}
          >
            <Icon icon="solar:close-circle-linear" size={12} />
          </button>
        </div>
      {/each}
    {/if}
  </div>
</div>

<style>
  .popover {
    position: absolute;
    bottom: calc(100% + 6px);
    right: 0;
    width: 360px;
    max-height: 60vh;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 12px 36px rgba(0, 0, 0, 0.5);
    display: flex;
    flex-direction: column;
    z-index: 1000;
    overflow: hidden;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 8px 6px 12px;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-subtle);
    flex-shrink: 0;
  }
  .title {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--text);
  }
  .count {
    font-size: 10px;
    font-family: var(--font-mono);
    color: var(--text-faint);
    background: var(--bg-elevated);
    padding: 1px 5px;
    border-radius: 8px;
    margin-right: auto;
  }
  .hd-btn {
    background: transparent;
    border: 1px solid var(--border-subtle);
    color: var(--text-muted);
    font-size: var(--text-xs);
    padding: 2px 8px;
    border-radius: 4px;
    cursor: pointer;
  }
  .hd-btn:hover:not(:disabled) {
    color: var(--text);
    border-color: var(--border);
  }
  .hd-btn:disabled {
    opacity: 0.4;
    cursor: default;
  }
  .hd-btn.icon {
    padding: 2px;
    width: 22px;
    height: 22px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-color: transparent;
  }

  .body {
    flex: 1;
    overflow: auto;
    padding: 4px;
  }
  .empty {
    padding: 18px 12px;
    color: var(--text-faint);
    font-size: var(--text-sm);
    text-align: center;
  }

  .row {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 8px 6px 8px 10px;
    border-radius: 5px;
    border-left: 2px solid transparent;
  }
  .row + .row {
    margin-top: 2px;
  }
  .row:hover {
    background: rgba(255, 255, 255, 0.04);
  }
  .row.n-error {
    border-left-color: var(--danger);
  }
  .row.n-warning {
    border-left-color: var(--warning);
  }
  .row.n-info {
    border-left-color: var(--info);
  }

  .content {
    flex: 1;
    min-width: 0;
  }
  .title-row {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
  .t-title {
    flex: 1;
    color: var(--text);
    font-weight: 600;
    font-size: var(--text-sm);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .t-time {
    color: var(--text-faint);
    font-size: 10px;
    font-family: var(--font-mono);
    flex-shrink: 0;
  }
  .t-body {
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    margin-top: 2px;
    max-height: 120px;
    overflow: auto;
    white-space: pre-wrap;
    word-break: break-word;
  }
  .action {
    margin-top: 6px;
    background: transparent;
    border: 1px solid var(--border-subtle);
    color: var(--text);
    font-size: var(--text-xs);
    padding: 2px 8px;
    border-radius: 4px;
    cursor: pointer;
  }
  .action:hover {
    border-color: var(--accent);
    color: var(--accent);
  }

  .dismiss {
    background: transparent;
    border: 0;
    color: var(--text-faint);
    cursor: pointer;
    padding: 2px;
    width: 20px;
    height: 20px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: 3px;
    flex-shrink: 0;
  }
  .dismiss:hover {
    color: var(--text);
    background: rgba(255, 255, 255, 0.08);
  }
</style>
