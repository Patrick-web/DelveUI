// CSS-variable-driven CodeMirror theme. Values reference the same tokens
// that theme-engine.ts writes to :root, so switching the app theme
// automatically retints the editor.
import { EditorView } from "@codemirror/view";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { tags as t } from "@lezer/highlight";

export const editorTheme = EditorView.theme(
  {
    "&": {
      color: "var(--text)",
      backgroundColor: "var(--bg)",
    },
    ".cm-content": {
      caretColor: "var(--accent)",
    },
    ".cm-cursor, .cm-dropCursor": {
      borderLeftColor: "var(--accent)",
    },
    "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection": {
      backgroundColor: "var(--accent-subtle)",
    },
    ".cm-activeLine": {
      backgroundColor: "var(--accent-subtle)",
    },
    ".cm-activeLineGutter": {
      backgroundColor: "var(--accent-subtle)",
      color: "var(--text)",
    },
    ".cm-gutters": {
      backgroundColor: "var(--bg-subtle)",
      color: "var(--text-faint)",
      borderRight: "1px solid var(--border-subtle)",
    },
    ".cm-lineNumbers .cm-gutterElement": {
      color: "var(--text-faint)",
    },
    ".cm-selectionMatch": {
      backgroundColor: "rgba(255, 204, 0, 0.18)",
    },
    ".cm-matchingBracket, .cm-nonmatchingBracket": {
      backgroundColor: "var(--border-subtle)",
      outline: "1px solid var(--accent)",
    },
    ".cm-panels": {
      backgroundColor: "var(--bg-elevated)",
      color: "var(--text)",
    },
    ".cm-panels.cm-panels-top": {
      borderBottom: "1px solid var(--border)",
    },
    ".cm-panels.cm-panels-bottom": {
      borderTop: "1px solid var(--border)",
    },
    ".cm-tooltip": {
      border: "1px solid var(--border)",
      backgroundColor: "var(--bg-elevated)",
      color: "var(--text)",
      borderRadius: "6px",
    },
    ".cm-tooltip .cm-tooltip-arrow:before": {
      borderTopColor: "var(--border)",
      borderBottomColor: "var(--border)",
    },
    ".cm-tooltip .cm-tooltip-arrow:after": {
      borderTopColor: "var(--bg-elevated)",
      borderBottomColor: "var(--bg-elevated)",
    },
    ".cm-tooltip-autocomplete": {
      "& > ul > li[aria-selected]": {
        backgroundColor: "var(--accent)",
        color: "#fff",
      },
    },
    ".cm-foldPlaceholder": {
      backgroundColor: "var(--bg-subtle)",
      color: "var(--text-muted)",
      border: "1px solid var(--border)",
      borderRadius: "4px",
      padding: "0 4px",
    },
    ".cm-searchMatch": {
      backgroundColor: "rgba(255, 204, 0, 0.25)",
      borderRadius: "2px",
    },
    ".cm-searchMatch.cm-searchMatch-selected": {
      backgroundColor: "rgba(255, 204, 0, 0.5)",
    },
  },
  { dark: true },
);

const highlightStyle = HighlightStyle.define([
  // keywords
  { tag: [t.keyword, t.controlKeyword, t.definitionKeyword, t.moduleKeyword, t.operatorKeyword, t.self, t.null], color: "var(--syn-keyword)" },
  // strings
  { tag: [t.string, t.special(t.string), t.character], color: "var(--syn-string)" },
  // numbers / constants
  { tag: [t.number, t.integer, t.float, t.bool, t.regexp, t.escape], color: "var(--syn-number)" },
  // function names (definitions and calls)
  { tag: [t.function(t.variableName), t.function(t.propertyName), t.definition(t.function(t.variableName))], color: "var(--syn-fn)" },
  // types / classes
  { tag: [t.typeName, t.className, t.namespace], color: "var(--warning)" },
  // comments
  { tag: [t.comment, t.lineComment, t.blockComment, t.docComment], color: "var(--syn-comment)", fontStyle: "italic" },
  // general names
  { tag: [t.variableName, t.propertyName, t.attributeName], color: "var(--text)" },
  // punctuation, operators
  { tag: [t.operator, t.punctuation, t.bracket, t.separator, t.derefOperator], color: "var(--text-muted)" },
  // meta
  { tag: [t.meta, t.documentMeta], color: "var(--text-faint)" },
  // invalid
  { tag: t.invalid, color: "var(--danger)" },
]);

export const editorHighlighting = syntaxHighlighting(highlightStyle);
