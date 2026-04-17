import { writable, derived, get } from "svelte/store";
import { Events } from "@wailsio/runtime";
import * as WorkspaceService from "../../bindings/github.com/jp/DelveUI/internal/services/workspaceservice";
import * as SessionService from "../../bindings/github.com/jp/DelveUI/internal/services/sessionservice";
import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";

export type LaunchConfig = {
  id: string;
  label: string;
  adapter?: string;
  request?: string;
  mode?: string;
  program?: string;
  cwd?: string;
  envFile?: string;
  env?: Record<string, string>;
  args?: string[];
  buildFlags?: string[];
  disabled?: boolean;
  disabledNote?: string;
  language?: string;
};

export type SessionInfo = {
  id: string;
  cfgId: string;
  label: string;
  state: string;
  port: number;
  pid: number;
};

export type StackFrame = {
  id: number;
  name: string;
  source?: { path: string; name?: string };
  line: number;
  column?: number;
};

export type Variable = {
  name: string;
  value: string;
  type?: string;
  variablesReference: number;
};

export type WorkspaceInfo = {
  root: string;
  debugFile: string;
  configs: LaunchConfig[];
  recents: { path: string; lastUsed: string }[];
  loadedOk: boolean;
  loadError?: string;
};

export type SessionEvent = {
  sessionId: string;
  kind: string;
  state?: string;
  output?: string;
  category?: string;
  threadId?: number;
  reason?: string;
  message?: string;
};

export const workspace = writable<WorkspaceInfo | null>(null);
export const sessions = writable<Record<string, SessionInfo>>({});
export const activeSessionId = writable<string | null>(null);

export const activeSession = derived(
  [sessions, activeSessionId],
  ([$s, $id]) => ($id ? $s[$id] : null),
);

// Shared frame selection — selectedFrameId is set by CallStackPanel, read by Variables/Source/Console
export const selectedFrameId = writable<number>(0);

// Manual source path — set by file tree / quick open, overridden by frame on stop
export const manualSourcePath = writable<string>("");

// Per-session state
type SessionState = {
  output: { cat: string; text: string }[];
  stack: StackFrame[];
  stoppedThread: number;
  breakpoints: Record<string, number[]>; // sourcePath → lines
};

export const sessionState = writable<Record<string, SessionState>>({});

export const selectedFrame = derived(
  [activeSessionId, sessionState, selectedFrameId],
  ([$sid, $ss, $fid]) => {
    const stack = $sid ? ($ss[$sid]?.stack ?? []) : [];
    return stack.find((f) => f.id === $fid) ?? stack[0] ?? null;
  },
);

function ensureSession(id: string) {
  sessionState.update((m) => {
    if (!m[id])
      m[id] = { output: [], stack: [], stoppedThread: 0, breakpoints: {} };
    return m;
  });
}

export async function refreshWorkspace() {
  const info = (await WorkspaceService.Info()) as any;
  workspace.set(info as WorkspaceInfo);
}

export async function refreshSessions() {
  const list = (await SessionService.List()) as any as SessionInfo[];
  const m: Record<string, SessionInfo> = {};
  for (const s of list) m[s.id] = s;
  sessions.set(m);
}

export async function openWorkspace(path: string) {
  const info = (await WorkspaceService.OpenWorkspace(path)) as any;
  workspace.set(info as WorkspaceInfo);
}

export async function pickDebugFile() {
  try {
    const info = (await WorkspaceService.PickDebugFile()) as any;
    workspace.set(info as WorkspaceInfo);
    // Auto-persist to debug files database
    if (info?.debugFile) {
      const { addDebugFile } = await import("./settings-store");
      await addDebugFile(info.debugFile).catch(() => {});
    }
  } catch (e: any) {
    const msg = String(e?.message ?? e);
    const { showError } = await import("./toast");
    showError("Could not load debug.json", msg);
  }
}

export async function pickWorkspaceFolder() {
  try {
    const info = (await WorkspaceService.PickWorkspaceFolder()) as any;
    workspace.set(info as WorkspaceInfo);
  } catch (e: any) {
    const msg = String(e?.message ?? e);
    const { showError } = await import("./toast");
    showError("Could not load workspace", msg);
  }
}

export async function openDebugFile(path: string) {
  try {
    const info = (await WorkspaceService.OpenDebugFile(path)) as any;
    workspace.set(info as WorkspaceInfo);
  } catch (e: any) {
    const msg = String(e?.message ?? e);
    const { showError } = await import("./toast");
    showError("Could not open file", msg);
  }
}

export async function startSession(cfgId: string) {
  try {
    const result = (await SessionService.Start(cfgId)) as any;
    const s = result.session as SessionInfo;
    if (s?.id) {
      sessions.update((m) => ({ ...m, [s.id]: s }));
      activeSessionId.set(s.id);
      ensureSession(s.id);
      // Send any pre-set breakpoints to the new session
      sendBreakpointsToSession(s.id).catch(() => {});
      // Focus the terminal tab for the new session
      import("./panels/layout").then(({ setActivePanel }) => {
        setActivePanel("right", "terminal");
      });
    }
    if (result.error) {
      const { showError } = await import("./toast");
      showError("Failed to start session", result.error);
    }
    return s;
  } catch (e: any) {
    const msg = String(e?.message ?? e);
    const { showError } = await import("./toast");
    showError("Failed to start session", msg);
    throw e;
  }
}

export async function restartSession(id: string) {
  try {
    removeSession(id);
    const result = (await SessionService.Restart(id)) as any;
    const s = result.session as SessionInfo;
    if (s?.id) {
      sessions.update((m) => ({ ...m, [s.id]: s }));
      activeSessionId.set(s.id);
      ensureSession(s.id);
      import("./panels/layout").then(({ setActivePanel }) => {
        setActivePanel("right", "terminal");
      });
    }
    if (result.error) {
      const { showError } = await import("./toast");
      showError("Restart failed", result.error);
    }
  } catch (e: any) {
    const { showError } = await import("./toast");
    showError("Restart failed", String(e?.message ?? e));
  }
}

export async function stopSession(id: string) {
  await SessionService.Stop(id);
  removeSession(id);
}

function removeSession(id: string) {
  sessions.update((m) => {
    const copy = { ...m };
    delete copy[id];
    return copy;
  });
  sessionState.update((m) => {
    const copy = { ...m };
    delete copy[id];
    return copy;
  });
  activeSessionId.update((cur) => {
    if (cur !== id) return cur;
    const remaining = Object.keys(get(sessions));
    return remaining.length > 0 ? remaining[0] : null;
  });
}

export async function control(
  action: "Continue" | "StepOver" | "StepIn" | "StepOut" | "Pause",
  id: string,
) {
  await (SessionService as any)[action](id);
}

// Global breakpoints store — persists independently of sessions
export const globalBreakpoints = writable<Record<string, number[]>>({});

export async function setBreakpoints(
  sessionId: string | null,
  sourcePath: string,
  lines: number[],
) {
  // Always update global store
  globalBreakpoints.update((m) => ({ ...m, [sourcePath]: lines }));

  // Also update per-session state if session exists
  if (sessionId) {
    sessionState.update((m) => {
      ensureSession(sessionId);
      m[sessionId].breakpoints[sourcePath] = lines;
      return { ...m };
    });
    try {
      return await SessionService.SetBreakpoints(sessionId, sourcePath, lines);
    } catch (e) {
      console.error("SetBreakpoints failed:", e);
    }
  }
}

// Send all global breakpoints to a session (called on session start)
export async function sendBreakpointsToSession(sessionId: string) {
  const bps = get(globalBreakpoints);
  for (const [sourcePath, lines] of Object.entries(bps)) {
    if (lines.length > 0) {
      try {
        await SessionService.SetBreakpoints(sessionId, sourcePath, lines);
        sessionState.update((m) => {
          ensureSession(sessionId);
          m[sessionId].breakpoints[sourcePath] = lines;
          return { ...m };
        });
      } catch {}
    }
  }
}

export async function fetchStack(sessionId: string) {
  const st = get(sessionState)[sessionId];
  const tid = st?.stoppedThread ?? 0;
  const resp = (await SessionService.StackTrace(sessionId, tid)) as any;
  const frames = (resp?.stackFrames ?? []) as StackFrame[];
  sessionState.update((m) => {
    ensureSession(sessionId);
    m[sessionId].stack = frames;
    return { ...m };
  });
  // Select top frame by default
  if (frames.length > 0) selectedFrameId.set(frames[0].id);
  return frames;
}

export async function fetchScopes(sessionId: string, frameId: number) {
  return (await SessionService.Scopes(sessionId, frameId)) as any;
}

export async function fetchVariables(sessionId: string, ref: number) {
  const r = (await SessionService.Variables(sessionId, ref)) as any;
  return (r?.variables ?? []) as Variable[];
}

export async function evaluate(
  sessionId: string,
  expr: string,
  frameId: number,
) {
  return (await SessionService.Evaluate(sessionId, expr, frameId)) as any;
}

export function clearSessionOutput(sessionId: string) {
  sessionState.update((m) => {
    if (m[sessionId]) m[sessionId] = { ...m[sessionId], output: [] };
    return { ...m };
  });
}

export async function readFile(path: string): Promise<string> {
  return (await FileService.ReadFile(path)) as string;
}

// Wire Wails events.
Events.On("session:event", async (ev: any) => {
  const e: SessionEvent = ev.data;
  console.debug("[session:event]", e);
  ensureSession(e.sessionId);
  // Auto-register a placeholder session so panes have something to display
  sessions.update((m) => {
    if (!m[e.sessionId]) {
      m[e.sessionId] = {
        id: e.sessionId,
        cfgId: "",
        label: "(unknown)",
        state: e.state ?? "idle",
        port: 0,
        pid: 0,
      };
      if (!get(activeSessionId)) activeSessionId.set(e.sessionId);
    }
    return m;
  });
  if (e.kind === "error" && e.message) {
    const { showError } = await import("./toast");
    showError("Debug session error", e.message);
    return;
  }
  if (e.kind === "state" && e.state) {
    if (e.state === "exited" || e.state === "error") {
      setTimeout(() => removeSession(e.sessionId), 1500);
      return;
    }
    sessions.update((m) => {
      if (m[e.sessionId]) m[e.sessionId] = { ...m[e.sessionId], state: e.state! };
      return m;
    });
  } else if (e.kind === "output") {
    sessionState.update((m) => {
      ensureSession(e.sessionId);
      m[e.sessionId].output.push({ cat: e.category ?? "output", text: e.output ?? "" });
      if (m[e.sessionId].output.length > 2000)
        m[e.sessionId].output.splice(0, m[e.sessionId].output.length - 2000);
      return { ...m };
    });
    // Detect port-in-use errors
    const text = e.output ?? "";
    const portMatch = text.match(/(?:listen|bind).*?(?::(\d+)).*?address already in use/i)
      || text.match(/address already in use.*?(?::(\d+))/i)
      || text.match(/bind: address already in use/i);
    if (portMatch) {
      const port = portMatch[1] ? parseInt(portMatch[1], 10) : 0;
      const sid = e.sessionId;
      const sess = get(sessions)[sid];
      const cfgId = sess?.cfgId ?? "";
      import("./toast").then(({ toasts, dismiss }) => {
        import("../../bindings/github.com/jp/DelveUI/internal/services/sessionservice").then(
          (SessionService) => {
            const id = Date.now();
            toasts.update((list) => [
              ...list,
              {
                id,
                kind: "warning" as const,
                title: port ? `Port ${port} already in use` : "Address already in use",
                body: "Another process is occupying the port. Kill it and retry?",
                action: {
                  label: "Kill & Retry",
                  run: async () => {
                    dismiss(id);
                    if (port) {
                      try {
                        await SessionService.KillPort(port);
                      } catch {}
                    }
                    // Stop the failed session then retry
                    try { await SessionService.Stop(sid); } catch {}
                    removeSession(sid);
                    if (cfgId) {
                      setTimeout(() => startSession(cfgId), 500);
                    }
                  },
                },
              },
            ]);
            setTimeout(() => dismiss(id), 15000);
          },
        );
      });
    }
  } else if (e.kind === "stopped") {
    sessionState.update((m) => {
      ensureSession(e.sessionId);
      m[e.sessionId].stoppedThread = e.threadId ?? 0;
      return { ...m };
    });
    // Reset frame selection + clear manual source so frame takes over
    selectedFrameId.set(0);
    manualSourcePath.set("");
    // Auto-switch to source panel when stopped
    import("./panels/layout").then(({ setActivePanel }) => {
      setActivePanel("right", "source");
    });
    sessions.update((m) => {
      if (m[e.sessionId]) m[e.sessionId] = { ...m[e.sessionId], state: "stopped" };
      return m;
    });
    // auto-refresh stack
    fetchStack(e.sessionId).catch(() => {});
  }
});

Events.On("workspace:changed", (ev: any) => {
  workspace.set(ev.data as WorkspaceInfo);
});

Events.On("switch-session", (ev: any) => {
  const sid = ev.data as string;
  if (sid) activeSessionId.set(sid);
});

Events.On("update:available", async (ev: any) => {
  const info = ev.data as any;
  if (!info?.available) return;
  const { toasts, dismiss } = await import("./toast");
  const id = Date.now();
  toasts.update((list) => [
    ...list,
    {
      id,
      kind: "info" as const,
      title: `Update available: v${info.latestVersion}`,
      body: `You're on v${info.currentVersion}. A new version is ready.`,
      action: {
        label: "View release",
        run: () => {
          dismiss(id);
          if (info.releaseUrl) window.open(info.releaseUrl, "_blank");
        },
      },
    },
  ]);
});
