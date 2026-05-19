<script lang="ts">
  import { onMount } from "svelte";
  import { Terminal } from "@xterm/xterm";
  import { FitAddon } from "@xterm/addon-fit";
  import { WebglAddon } from "@xterm/addon-webgl";
  import { SearchAddon } from "@xterm/addon-search";
  import "@xterm/xterm/css/xterm.css";

  export let lines: { cat: string; text: string }[] = [];
  export let filterMode: string = "all";
  export let searchQuery: string = "";
  export let matchCount: number = 0;

  let container: HTMLDivElement;
  let terminal: Terminal;
  let fitAddon: FitAddon;
  let searchAddon: SearchAddon;
  let syncedLen = 0;
  let lastFilter = filterMode;

  onMount(() => {
    terminal = new Terminal({
      scrollback: 10000,
      disableStdin: true,
      cursorBlink: false,
      convertEol: true,
      fontFamily:
        'ui-monospace, "SF Mono", "IBM Plex Mono", "JetBrains Mono", SFMono-Regular, Menlo, monospace',
      fontSize: 12,
      theme: {
        background: "#0f1115",
        foreground: "#d8dbe1",
        cursor: "#4d9cff",
        black: "#282c34",
        red: "#e06c75",
        green: "#98c379",
        yellow: "#e5c07b",
        blue: "#61afef",
        magenta: "#c678dd",
        cyan: "#56b6c2",
        white: "#abb2bf",
        brightBlack: "#5c6370",
        brightRed: "#e06c75",
        brightGreen: "#98c379",
        brightYellow: "#e5c07b",
        brightBlue: "#61afef",
        brightMagenta: "#c678dd",
        brightCyan: "#56b6c2",
        brightWhite: "#ffffff",
        selectionBackground: "#4d9cff40",
      },
    });

    fitAddon = new FitAddon();
    searchAddon = new SearchAddon();
    terminal.loadAddon(fitAddon);
    terminal.loadAddon(searchAddon);

    try {
      terminal.loadAddon(new WebglAddon());
    } catch {
      // canvas renderer fallback
    }

    terminal.open(container);

    for (const l of lines) terminal.write(l.text);
    syncedLen = lines.length;

    const ro = new ResizeObserver(() => {
      try {
        fitAddon.fit();
      } catch {}
    });
    ro.observe(container);

    searchAddon.onDidChangeResults((e) => {
      matchCount = e.resultCount;
    });

    return () => {
      ro.disconnect();
      terminal.dispose();
    };
  });

  $: if (terminal && filterMode !== lastFilter) {
    terminal.reset();
    for (const l of lines) terminal.write(l.text);
    syncedLen = lines.length;
    lastFilter = filterMode;
    if (searchQuery) searchAddon.findNext(searchQuery);
  }

  $: if (terminal && lines.length > syncedLen) {
    for (let i = syncedLen; i < lines.length; i++) terminal.write(lines[i].text);
    syncedLen = lines.length;
  }

  $: if (terminal && lines.length < syncedLen) {
    terminal.clear();
    syncedLen = 0;
    lastFilter = filterMode;
    for (const l of lines) terminal.write(l.text);
    syncedLen = lines.length;
  }

  $: if (terminal && searchAddon) {
    if (searchQuery) {
      searchAddon.findNext(searchQuery);
    } else {
      searchAddon.clearDecorations();
      matchCount = 0;
    }
  }
</script>

<div class="term-container" bind:this={container}></div>

<style>
  .term-container {
    flex: 1;
    overflow: hidden;
  }
</style>
