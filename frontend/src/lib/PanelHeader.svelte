<script lang="ts">
  export let title: string;
</script>

<div class="panel-header">
  <span class="title">{title}</span>
  <span class="actions">
    <slot />
  </span>
</div>

<style>
  .panel-header {
    display: flex;
    align-items: center;
    height: 22px;
    padding: 0 var(--space-2);
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg);
    flex-shrink: 0;
  }
  .title {
    flex: 1;
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.5px;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .actions {
    display: flex;
    gap: 2px;
  }
</style>
