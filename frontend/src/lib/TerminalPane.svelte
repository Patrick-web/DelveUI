<script lang="ts">
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import { Terminal } from "@xterm/xterm";
  import type { ITheme } from "@xterm/xterm";
  import { FitAddon } from "@xterm/addon-fit";
  import { WebglAddon } from "@xterm/addon-webgl";
  import { SearchAddon } from "@xterm/addon-search";
  import "@xterm/xterm/css/xterm.css";
  import { currentTheme } from "./theme-engine";

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

  function themeToITheme(theme: any): ITheme {
    const t = theme?.style?.terminal || {};
    const accent = (theme?.style?.accent || "#4d9cff") + "40";
    return {
      background: t.background || theme?.style?.bgSubtle || "#0f1115",
      foreground: t.foreground || theme?.style?.text || "#d8dbe1",
      cursor: t.cursor || theme?.style?.accent || "#4d9cff",
      black: t.black || "#282c34",
      red: t.red || "#e06c75",
      green: t.green || "#98c379",
      yellow: t.yellow || "#e5c07b",
      blue: t.blue || "#61afef",
      magenta: t.magenta || "#c678dd",
      cyan: t.cyan || "#56b6c2",
      white: t.white || "#abb2bf",
      brightBlack: t.brightBlack || "#5c6370",
      brightRed: t.brightRed || "#e06c75",
      brightGreen: t.brightGreen || "#98c379",
      brightYellow: t.brightYellow || "#e5c07b",
      brightBlue: t.brightBlue || "#61afef",
      brightMagenta: t.brightMagenta || "#c678dd",
      brightCyan: t.brightCyan || "#56b6c2",
      brightWhite: t.brightWhite || "#ffffff",
      selectionBackground: accent,
    };
  }

  let unsubTheme: () => void;

  onMount(() => {
    terminal = new Terminal({
      scrollback: 10000,
      disableStdin: true,
      cursorBlink: false,
      convertEol: true,
      fontFamily:
        'ui-monospace, "SF Mono", "IBM Plex Mono", "JetBrains Mono", SFMono-Regular, Menlo, monospace',
      fontSize: 12,
      theme: themeToITheme(get(currentTheme)),
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

    unsubTheme = currentTheme.subscribe((t) => {
      if (terminal) terminal.options.theme = { ...themeToITheme(t) };
    });

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
      unsubTheme();
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
