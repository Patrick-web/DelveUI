<script lang="ts">
  import { afterUpdate } from "svelte";
  import { AnsiUp } from "ansi_up";

  export let lines: { cat: string; text: string }[] = [];
  export let filter: (cat: string) => boolean = () => true;

  const ansi = new AnsiUp();
  ansi.use_classes = true;

  let el: HTMLDivElement;
  let atBottom = true;

  $: rendered = lines
    .filter((l) => filter(l.cat))
    .map((l) => `<span class="cat-${l.cat}">${ansi.ansi_to_html(l.text)}</span>`)
    .join("");

  afterUpdate(() => {
    if (el && atBottom) el.scrollTop = el.scrollHeight;
  });

  function onScroll() {
    if (!el) return;
    atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
  }
</script>

<div class="term" bind:this={el} on:scroll={onScroll}>
  {@html rendered}
</div>

<style>
  .term {
    flex: 1;
    overflow: auto;
    padding: var(--space-2) var(--space-3);
    font-family: var(--font-terminal);
    font-size: var(--text-term, var(--text-sm));
    line-height: var(--lh-standard);
    white-space: pre-wrap;
    background: var(--term-background);
    color: var(--term-foreground);
  }
</style>
