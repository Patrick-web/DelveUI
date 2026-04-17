<script lang="ts">
  import { afterUpdate } from "svelte";
  import { AnsiUp } from "ansi_up";

  export let lines: { cat: string; text: string }[] = [];
  export let filter: (cat: string) => boolean = () => true;
  export let searchQuery: string = "";
  export let matchCount: number = 0;

  const ansi = new AnsiUp();
  ansi.use_classes = true;

  let el: HTMLDivElement;
  let atBottom = true;

  $: rendered = buildRendered(lines, searchQuery);

  function buildRendered(lines: { cat: string; text: string }[], query: string): string {
    let count = 0;
    const html = lines
      .filter((l) => filter(l.cat))
      .map((l) => {
        let content = ansi.ansi_to_html(l.text);
        if (query && query.length > 0) {
          const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
          const re = new RegExp(`(${escaped})`, "gi");
          content = content.replace(re, (m) => {
            count++;
            return `<mark class="search-hl">${m}</mark>`;
          });
        }
        return `<span class="cat-${l.cat}">${content}</span>`;
      })
      .join("");
    matchCount = count;
    return html;
  }

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
  .term :global(.search-hl) {
    background: rgba(255, 204, 0, 0.3);
    color: inherit;
    border-radius: 2px;
    padding: 0 1px;
  }
</style>
