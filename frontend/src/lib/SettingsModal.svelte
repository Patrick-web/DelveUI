<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { EditorView, keymap, lineNumbers, highlightActiveLine } from "@codemirror/view";
  import { EditorState, Compartment } from "@codemirror/state";
  import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
  import { json } from "@codemirror/lang-json";
  import { oneDark } from "@codemirror/theme-one-dark";
  import { vim } from "@replit/codemirror-vim";
  import { bracketMatching, indentOnInput, foldGutter, foldKeymap } from "@codemirror/language";
  import Icon from "./Icon.svelte";
  import { workspace, refreshWorkspace } from "./store";
  import { appSettings } from "./settings-store";
  import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";
  import * as WorkspaceService from "../../bindings/github.com/jp/DelveUI/internal/services/workspaceservice";

  export let open = false;

  let container: HTMLDivElement;
  let view: EditorView | null = null;
  const vimCompartment = new Compartment();
  let vimEnabled = $appSettings.vimMode ?? false;
  let path = "";
  let dirty = false;
  let saving = false;
  let error = "";

  $: if (open) loadFile();
  $: if (!open && view) disposeEditor();

  async function loadFile() {
    error = "";
    path = $workspace?.debugFile ?? "";
    if (!path) {
      error = "No debug.json loaded. Open one first.";
      return;
    }
    try {
      const content = (await FileService.ReadFile(path)) as string;
      // Wait a tick for container to mount
      await new Promise((r) => requestAnimationFrame(() => r(null)));
      if (view) view.destroy();
      view = new EditorView({
        parent: container,
        state: EditorState.create({
          doc: content,
          extensions: [
            vimCompartment.of(vimEnabled ? vim() : []),
            lineNumbers(),
            foldGutter(),
            highlightActiveLine(),
            history(),
            bracketMatching(),
            indentOnInput(),
            json(),
            oneDark,
            keymap.of([...defaultKeymap, ...historyKeymap, ...foldKeymap, indentWithTab]),
            EditorView.updateListener.of((u) => {
              if (u.docChanged) dirty = true;
            }),
            EditorView.theme({
              "&": { height: "100%", fontSize: "13px" },
              ".cm-content": { fontFamily: "var(--font-mono)" },
              ".cm-gutters": { backgroundColor: "var(--bg-subtle)", borderRight: "1px solid var(--border-subtle)" },
              "&.cm-focused": { outline: "none" },
            }),
          ],
        }),
      });
      dirty = false;
    } catch (e: any) {
      error = String(e);
    }
  }

  function toggleVim() {
    vimEnabled = !vimEnabled;
    if (view) {
      view.dispatch({ effects: vimCompartment.reconfigure(vimEnabled ? vim() : []) });
    }
  }

  async function save() {
    if (!view || !path) return;
    saving = true;
    error = "";
    try {
      const content = view.state.doc.toString();
      await FileService.WriteFile(path, content);
      await WorkspaceService.OpenDebugFile(path); // reload configs
      await refreshWorkspace();
      dirty = false;
    } catch (e: any) {
      error = String(e);
    } finally {
      saving = false;
    }
  }

  function disposeEditor() {
    if (view) {
      view.destroy();
      view = null;
    }
  }

  function close() {
    open = false;
  }

  function onBackdrop(e: MouseEvent) {
    if (e.target === e.currentTarget) close();
  }

  function onKey(e: KeyboardEvent) {
    if (!open) return;
    if (e.key === "Escape") {
      close();
    } else if ((e.metaKey || e.ctrlKey) && e.key === "s") {
      e.preventDefault();
      save();
    }
  }

  onMount(() => {
    window.addEventListener("keydown", onKey);
  });
  onDestroy(() => {
    window.removeEventListener("keydown", onKey);
    disposeEditor();
  });
</script>

{#if open}
  <div
    class="backdrop"
    role="presentation"
    on:click={onBackdrop}
    on:keydown={(e) => e.key === "Escape" && close()}
  >
    <div class="modal" role="dialog" aria-modal="true" tabindex="-1" on:click|stopPropagation on:keydown|stopPropagation>
      <header>
        <div class="title">
          <Icon icon="solar:settings-bold" size={14} />
          <span>Settings</span>
          <span class="path">{path}</span>
          {#if dirty}<span class="dirty">●</span>{/if}
        </div>
        <div class="actions">
          <label class="vim-toggle">
            <input type="checkbox" checked={vimEnabled} on:change={toggleVim} />
            <span>Vim mode</span>
          </label>
          <button class="btn primary" on:click={save} disabled={!dirty || saving || !path}>
            <Icon icon="solar:diskette-bold" size={12} />
            {saving ? "Saving…" : "Save"}
          </button>
          <button class="btn icon" title="Close (Esc)" on:click={close}>
            <Icon icon="solar:close-circle-linear" size={14} />
          </button>
        </div>
      </header>
      {#if error}
        <div class="error">{error}</div>
      {/if}
      <div class="editor" bind:this={container}></div>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 800;
    display: flex;
    align-items: stretch;
    justify-content: center;
    padding: 40px;
  }
  .modal {
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    width: 100%;
    max-width: 1100px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.5);
  }
  header {
    display: flex;
    align-items: center;
    padding: 0 var(--space-3);
    height: 40px;
    border-bottom: 1px solid var(--border);
    background: var(--bg);
    flex-shrink: 0;
    gap: var(--space-3);
  }
  .title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text);
    flex: 1;
    min-width: 0;
  }
  .path {
    color: var(--text-faint);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dirty {
    color: var(--warning);
    font-size: 10px;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .vim-toggle {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--text-xs);
    color: var(--text-muted);
    cursor: pointer;
    user-select: none;
  }
  .vim-toggle input {
    accent-color: var(--accent);
  }
  .error {
    padding: var(--space-2) var(--space-3);
    color: var(--danger);
    background: rgba(224, 108, 117, 0.1);
    border-bottom: 1px solid var(--border);
    font-size: var(--text-sm);
    font-family: var(--font-mono);
  }
  .editor {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    background: var(--bg-subtle);
  }
  .editor :global(.cm-editor) {
    height: 100%;
  }
</style>
