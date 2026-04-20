// Extracts Go symbols from a CodeMirror EditorState using the Lezer
// grammar that ships with @codemirror/lang-go.
//
// Lezer Go node names we care about (see go.grammar):
//   FunctionDecl  -> "func" DefName ...      ← top-level function
//   MethodDecl    -> "func" Parameters FieldName ...  ← method on a type
//   TypeDecl      -> "type" TypeSpec | SpecList<TypeSpec>
//   TypeSpec      -> DefName ... type        ← actual named type
import type { EditorState } from "@codemirror/state";
import { syntaxTree } from "@codemirror/language";

export type SymbolKind = "func" | "method" | "type";

export type GoSymbol = {
  name: string;          // display name — includes receiver for methods ("T.M")
  kind: SymbolKind;
  line: number;          // 1-based
  pos: number;           // document offset of the declaration
};

export function extractGoSymbols(state: EditorState): GoSymbol[] {
  const out: GoSymbol[] = [];
  const tree = syntaxTree(state);
  const text = (from: number, to: number) => state.doc.sliceString(from, to);

  tree.iterate({
    enter: (node) => {
      if (node.name === "FunctionDecl") {
        const nameNode = node.node.getChild("DefName");
        if (nameNode) {
          out.push({
            name: text(nameNode.from, nameNode.to),
            kind: "func",
            line: state.doc.lineAt(node.from).number,
            pos: node.from,
          });
        }
        // Don't descend — body symbols aren't shown in the picker.
        return false;
      }
      if (node.name === "MethodDecl") {
        const nameNode = node.node.getChild("FieldName");
        const recv = node.node.getChild("Parameters");
        if (nameNode) {
          const methodName = text(nameNode.from, nameNode.to);
          let recvType = "";
          if (recv) {
            // Parameters text looks like "(r *Type)" or "(Type)" or "(r Type[P])"
            const raw = text(recv.from, recv.to);
            const m = raw.match(/\(\s*(?:\w+\s+)?\*?(\w+)/);
            if (m) recvType = m[1];
          }
          out.push({
            name: recvType ? `${recvType}.${methodName}` : methodName,
            kind: "method",
            line: state.doc.lineAt(node.from).number,
            pos: node.from,
          });
        }
        return false;
      }
      if (node.name === "TypeSpec") {
        const nameNode = node.node.getChild("DefName");
        if (nameNode) {
          out.push({
            name: text(nameNode.from, nameNode.to),
            kind: "type",
            line: state.doc.lineAt(node.from).number,
            pos: node.from,
          });
        }
        return false;
      }
    },
  });
  return out;
}

// Returns the innermost function/method containing `pos`, or null if none.
export function enclosingFunction(state: EditorState, pos: number): GoSymbol | null {
  const tree = syntaxTree(state);
  let result: GoSymbol | null = null;
  const text = (from: number, to: number) => state.doc.sliceString(from, to);

  tree.iterate({
    enter: (node) => {
      if (node.from > pos || node.to < pos) return false; // not containing
      if (node.name === "FunctionDecl") {
        const nameNode = node.node.getChild("DefName");
        if (nameNode) {
          result = {
            name: text(nameNode.from, nameNode.to),
            kind: "func",
            line: state.doc.lineAt(node.from).number,
            pos: node.from,
          };
        }
      } else if (node.name === "MethodDecl") {
        const nameNode = node.node.getChild("FieldName");
        const recv = node.node.getChild("Parameters");
        if (nameNode) {
          const methodName = text(nameNode.from, nameNode.to);
          let recvType = "";
          if (recv) {
            const raw = text(recv.from, recv.to);
            const m = raw.match(/\(\s*(?:\w+\s+)?\*?(\w+)/);
            if (m) recvType = m[1];
          }
          result = {
            name: recvType ? `${recvType}.${methodName}` : methodName,
            kind: "method",
            line: state.doc.lineAt(node.from).number,
            pos: node.from,
          };
        }
      }
    },
  });
  return result;
}
