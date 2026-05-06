import { writable, derived } from "svelte/store";

export type ToastKind = "error" | "warning" | "info";

export type Toast = {
  id: number;
  kind: ToastKind;
  title: string;
  body?: string;
  action?: { label: string; run: () => void };
};

// A notification is a toast that has been written to history. Toasts come and
// go (auto-dismiss after 10s); notifications stick around so the user can
// review what fired earlier and re-run the action.
export type Notification = {
  id: number;
  kind: ToastKind;
  title: string;
  body?: string;
  action?: { label: string; run: () => void };
  ts: number;
  read: boolean;
};

export const toasts = writable<Toast[]>([]);
export const notifications = writable<Notification[]>([]);

// Cap history so a noisy session doesn't grow the array unbounded.
const MAX_HISTORY = 100;

let nextId = 1;

function push(t: Toast) {
  toasts.update((list) => [...list, t]);
  notifications.update((list) => {
    const next: Notification = {
      id: t.id,
      kind: t.kind,
      title: t.title,
      body: t.body,
      action: t.action,
      ts: Date.now(),
      read: false,
    };
    return [next, ...list].slice(0, MAX_HISTORY);
  });
  setTimeout(() => dismiss(t.id), 10000);
}

// dismiss only removes the live toast — the notification remains in history
// so the user can find it later via the bell.
export function dismiss(id: number) {
  toasts.update((list) => list.filter((t) => t.id !== id));
}

export function dismissNotification(id: number) {
  notifications.update((list) => list.filter((n) => n.id !== id));
}

export function clearNotifications() {
  notifications.set([]);
}

export function markNotificationsRead() {
  notifications.update((list) => list.map((n) => (n.read ? n : { ...n, read: true })));
}

export const unreadCount = derived(notifications, ($n) => $n.filter((x) => !x.read).length);

// showError: emits an error toast and history entry. When `sessionId` is
// provided the action button switches to that session and opens the Debug
// Console so the user can inspect the underlying output that produced the
// error (e.g. dlv stderr).
export function showError(title: string, body?: string, sessionId?: string) {
  push({
    id: nextId++,
    kind: "error",
    title,
    body,
    action: {
      label: "Show in Debug Console",
      run: () => revealInDebugConsole(sessionId),
    },
  });
}

export function showInfo(title: string, body?: string) {
  push({ id: nextId++, kind: "info", title, body });
}

export function showWarning(title: string, body?: string) {
  push({ id: nextId++, kind: "warning", title, body });
}

// revealInDebugConsole: focuses the given session (if any) and switches the
// center area to the Debug Console tab. Exported so the notifications popover
// can re-trigger an action even after the toast has auto-dismissed.
export async function revealInDebugConsole(sessionId?: string) {
  if (sessionId) {
    const { activeSessionId } = await import("./store");
    activeSessionId.set(sessionId);
  }
  const { setCenterActive } = await import("./panels/layout");
  setCenterActive("console");
}
