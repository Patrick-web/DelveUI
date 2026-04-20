import { writable } from "svelte/store";

export type ToastKind = "error" | "warning" | "info";

export type Toast = {
  id: number;
  kind: ToastKind;
  title: string;
  body?: string;
  action?: { label: string; run: () => void };
};

export const toasts = writable<Toast[]>([]);

let nextId = 1;

function push(t: Toast) {
  toasts.update((list) => [...list, t]);
  setTimeout(() => dismiss(t.id), 10000);
}

export function dismiss(id: number) {
  toasts.update((list) => list.filter((t) => t.id !== id));
}

export function showError(title: string, body?: string) {
  push({
    id: nextId++,
    kind: "error",
    title,
    body,
    action: {
      label: "Show Terminal",
      run: () => {
        import("./panels/layout").then(({ showBottomTab }) => {
          showBottomTab("terminal");
        });
      },
    },
  });
}

export function showInfo(title: string, body?: string) {
  push({ id: nextId++, kind: "info", title, body });
}

export function showWarning(title: string, body?: string) {
  push({ id: nextId++, kind: "warning", title, body });
}
