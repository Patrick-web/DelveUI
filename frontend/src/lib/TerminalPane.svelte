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

  // Cache ansi→html and the wrapping span per line object. Lines are pushed
  // once and never mutated by the store, so identity is stable for their lifetime.
  // Without this, every output event re-parses every history line — the dominant
  // cost on long-running, chatty sessions.
  type Line = { cat: string; text: string };
  const ansiCache = new WeakMap<Line, string>();
  const wrappedCache = new WeakMap<Line, string>(); // `<span class="cat-X">…</span>`

  function wrappedFor(l: Line): string {
    let w = wrappedCache.get(l);
    if (w !== undefined) return w;
    let h = ansiCache.get(l);
    if (h === undefined) {
      h = ansi.ansi_to_html(l.text);
      ansiCache.set(l, h);
    }
    w = `<span class="cat-${l.cat}">${h}</span>`;
    wrappedCache.set(l, w);
    return w;
  }

  $: rendered = buildRendered(lines, searchQuery);

  function buildRendered(lines: Line[], query: string): string {
    // Hot path: no search query. Pure cache lookup + concat.
    if (!query) {
      matchCount = 0;
      let out = "";
      for (let i = 0; i < lines.length; i++) {
        const l = lines[i];
        if (!filter(l.cat)) continue;
        out += wrappedFor(l);
      }
      return out;
    }

    // Search path: highlight matches on top of cached ansi html.
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const re = new RegExp(`(${escaped})`, "gi");
    let count = 0;
    let out = "";
    for (let i = 0; i < lines.length; i++) {
      const l = lines[i];
      if (!filter(l.cat)) continue;
      let h = ansiCache.get(l);
      if (h === undefined) {
        h = ansi.ansi_to_html(l.text);
        ansiCache.set(l, h);
      }
      const highlighted = h.replace(re, (m) => {
        count++;
        return `<mark class="search-hl">${m}</mark>`;
      });
      out += `<span class="cat-${l.cat}">${highlighted}</span>`;
    }
    matchCount = count;
    return out;
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
