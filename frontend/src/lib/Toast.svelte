<script lang="ts">
  import Icon from "./Icon.svelte";
  import { toasts, dismiss } from "./toast";
</script>

<div class="stack">
  {#each $toasts as t (t.id)}
    <div class="toast toast-{t.kind}">
      <Icon
        icon={t.kind === "error"
          ? "solar:shield-warning-bold"
          : t.kind === "warning"
            ? "solar:danger-triangle-bold"
            : "solar:info-circle-bold"}
        size={16}
        color={t.kind === "error"
          ? "var(--danger)"
          : t.kind === "warning"
            ? "var(--warning)"
            : "var(--info)"}
      />
      <div class="content">
        <div class="title">{t.title}</div>
        {#if t.body}<div class="body">{t.body}</div>{/if}
      </div>
      {#if t.action}
        <button
          class="btn outlined"
          on:click={() => {
            t.action?.run();
            dismiss(t.id);
          }}
        >
          {t.action.label}
        </button>
      {/if}
      <button class="btn icon" on:click={() => dismiss(t.id)} title="Dismiss">
        <Icon icon="solar:close-circle-linear" size={14} />
      </button>
    </div>
  {/each}
</div>

<style>
  .stack {
    position: fixed;
    bottom: 32px;
    right: 16px;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    z-index: 950;
    pointer-events: none;
  }
  .toast {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-left: 3px solid var(--danger);
    border-radius: var(--radius-md);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
    min-width: 360px;
    max-width: 520px;
    pointer-events: auto;
    font-size: var(--text-sm);
  }
  .toast-warning { border-left-color: var(--warning); }
  .toast-info { border-left-color: var(--info); }
  .content { flex: 1; min-width: 0; }
  .title { color: var(--text); font-weight: 600; }
  .body {
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    margin-top: var(--space-1);
    white-space: pre-wrap;
    word-break: break-word;
    max-height: 200px;
    overflow: auto;
  }
</style>
