import { writable, derived, get } from "svelte/store";
import { Events } from "@wailsio/runtime";
import * as WorkspaceService from "../../bindings/github.com/jp/DelveUI/internal/services/workspaceservice";
import * as SessionService from "../../bindings/github.com/jp/DelveUI/internal/services/sessionservice";
import * as FileService from "../../bindings/github.com/jp/DelveUI/internal/services/fileservice";
import * as DiscoveryService from "../../bindings/github.com/jp/DelveUI/internal/discovery/service";
import { wrapHandler } from "./diagnostics";
import { bumpRecency } from "./recency-store";

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
  processId?: number;
  envFiles?: string[];
};

export type SessionInfo = {
  id: string;
  cfgId: string;
  label: string;
  state: string;
  port: number;
  pid: number;
  cfg?: LaunchConfig;
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
  loadedOk: boolean;
  loadError?: string;
};

export type SessionEvent = {
  sessionId: string;
  cfgId?: string;
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

// Scroll-to-line request — set when navigating to a breakpoint, consumed by SourcePanel
export const scrollToLineRequest = writable<number>(0);

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
    // The backend persists the picked folder as a project entry, so refresh
    // the project list so the ProjectSwitcher and Welcome page see it.
    if (info?.root) {
      const { loadDebugFiles } = await import("./settings-store");
      await loadDebugFiles().catch(() => {});
    }
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

// removeTerminatedForCfg cleans out any old error/exited session entries for
// the given cfg before a fresh start. Without this, the sessions/sessionState
// stores would accumulate one ghost per failed retry — invisible (because
// SessionsPanel only renders the latest session per cfgId), but kept in
// memory and confusing to debug.
function removeTerminatedForCfg(cfgId: string) {
  if (!cfgId) return;
  for (const s of Object.values(get(sessions))) {
    if (s.cfgId === cfgId && (s.state === "error" || s.state === "exited")) {
      removeSession(s.id);
    }
  }
}

export async function startSession(cfgId: string) {
  removeTerminatedForCfg(cfgId);
  bumpRecency(cfgId);
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
  const prev = get(sessions)[id];
  bumpRecency(prev?.cfgId);
  try {
    removeSession(id);
    const result = (await SessionService.Restart(id)) as any;
    const s = result.session as SessionInfo;
    if (s?.id) {
      sessions.update((m) => ({ ...m, [s.id]: s }));
      activeSessionId.set(s.id);
      ensureSession(s.id);
      sendBreakpointsToSession(s.id).catch(() => {});
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

// Dismiss a terminated session from the UI. Used by the Sessions / Run panels
// when the user is done inspecting a failed or exited session. For sessions
// that are still running we route through stopSession instead, so we don't
// orphan a live dlv process in the manager.
export async function dismissSession(id: string) {
  const s = get(sessions)[id];
  if (s && (s.state === "running" || s.state === "stopped" || s.state === "starting")) {
    await stopSession(id);
    return;
  }
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

export async function cleanDebugBinaries() {
  try {
    const result = (await SessionService.CleanDebugBinaries()) as any;
    const { showInfo } = await import("./toast");
    const count = result?.count ?? 0;
    if (count === 0) {
      showInfo("No debug binaries found", result?.dir ?? "");
    } else {
      showInfo(
        `Cleaned ${count} debug binary file${count === 1 ? "" : "s"}`,
        result?.dir ?? "",
      );
    }
  } catch (e: any) {
    const { showError } = await import("./toast");
    showError("Failed to clean debug binaries", String(e?.message ?? e));
  }
}

export async function control(
  action: "Continue" | "StepOver" | "StepIn" | "StepOut" | "Pause",
  id: string,
) {
  await (SessionService as any)[action](id);
}

// --- Discovery: auto-discovered run/test/attach targets ---

export type RunTarget = {
  id: string;
  provider: string;
  kind: "run" | "test" | "benchmark" | "example" | "attach" | string;
  label: string;
  description?: string;
  dir: string;
  program: string;
  args?: string[];
  env?: Record<string, string>;
  envFiles?: string[];
  pid?: number;
  sourceFile?: string;
  sourceLine?: number;
};

export const runTargets = writable<RunTarget[]>([]);
export const targetsLoading = writable<boolean>(false);
export const targetsLastScanned = writable<Date | null>(null);

export async function refreshTargets() {
  try {
    targetsLoading.set(true);
    const list = (await DiscoveryService.Refresh()) as any as RunTarget[];
    runTargets.set(list ?? []);
    targetsLastScanned.set(new Date());
  } catch (e: any) {
    const msg = String(e?.message ?? e);
    // "no workspace open" is the only expected error path; show a quieter
    // notice rather than an error toast so users opening DelveUI for the
    // first time aren't startled.
    if (!/no workspace open/i.test(msg)) {
      const { showError } = await import("./toast");
      showError("Failed to discover run targets", msg);
    }
  } finally {
    targetsLoading.set(false);
  }
}

// Discovery-launched sessions are virtual (no entry in workspace.configs), so
// the session:event handler skips registering them in the sessions store. We
// register them ourselves from the launch result, mirroring how startSession
// handles SessionService.Start.
function registerLaunchedSession(result: any, errorTitle: string) {
  if (result?.error) {
    import("./toast").then(({ showError }) => showError(errorTitle, result.error, result?.sessionId));
  }
  const sid = result?.sessionId as string | undefined;
  if (!sid) return undefined;
  removeTerminatedForCfg(result?.cfgId ?? "");
  const info: SessionInfo = {
    id: sid,
    cfgId: result.cfgId ?? "",
    label: result.label ?? "",
    state: result.state || "starting",
    port: result.port ?? 0,
    pid: result.pid ?? 0,
    cfg: result.cfg as LaunchConfig | undefined,
  };
  sessions.update((m) => ({ ...m, [sid]: info }));
  activeSessionId.set(sid);
  // ensureSession is only called from the session:event handler normally; do
  // it here so panes that depend on per-session state can render right away.
  sessionState.update((m) => {
    if (!m[sid]) m[sid] = { output: [], stack: [], stoppedThread: 0, breakpoints: {} };
    return m;
  });
  sendBreakpointsToSession(sid).catch(() => {});
  // Switch to terminal so output is visible.
  import("./panels/layout").then(({ setActivePanel }) => {
    setActivePanel("right", "terminal");
  });
  return sid;
}

export async function launchTarget(targetId: string) {
  bumpRecency(targetId);
  try {
    const result = (await DiscoveryService.Launch(targetId)) as any;
    return registerLaunchedSession(result, "Failed to launch target");
  } catch (e: any) {
    const { showError } = await import("./toast");
    showError("Failed to launch target", String(e?.message ?? e));
    throw e;
  }
}

export async function attachToProcess(pid: number) {
  try {
    const result = (await DiscoveryService.LaunchProcess(pid)) as any;
    return registerLaunchedSession(result, `Failed to attach to PID ${pid}`);
  } catch (e: any) {
    const { showError } = await import("./toast");
    showError(`Failed to attach to PID ${pid}`, String(e?.message ?? e));
    throw e;
  }
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

// Send all global breakpoints to a session (called on session start),
// then signal configurationDone so dlv resumes execution. The DAP spec
// requires this ordering — breakpoints set after configurationDone won't
// fire on code that has already executed.
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
  try {
    await SessionService.ConfigurationDone(sessionId);
  } catch (e) {
    console.error("ConfigurationDone failed:", e);
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
Events.On("session:event", wrapHandler("session:event", async (ev: any) => {
  const e: SessionEvent = ev.data;
  console.debug("[session:event]", e);
  ensureSession(e.sessionId);
  // Auto-register a placeholder session so panes have something to display
  // before SessionService.Start() resolves. The event carries cfgId so we
  // can look up the proper label and let the Run picker hide the launching
  // config — avoids a brief "(unknown)" row alongside the still-clickable cfg.
  sessions.update((m) => {
    if (!m[e.sessionId]) {
      const cfgId = e.cfgId ?? "";
      const ws = get(workspace);
      const cfg = cfgId ? ws?.configs?.find((c) => c.id === cfgId) : undefined;
      // If we can't resolve a cfg, skip the placeholder. Start() will
      // register the real session shortly; better empty than "(unknown)".
      if (!cfg) return m;
      m[e.sessionId] = {
        id: e.sessionId,
        cfgId,
        label: cfg.label,
        state: e.state ?? "starting",
        port: 0,
        pid: 0,
      };
      if (!get(activeSessionId)) activeSessionId.set(e.sessionId);
    }
    return m;
  });
  if (e.kind === "error" && e.message) {
    // Mirror the error into the session output so it shows up in both the
    // Terminal panel (cat-filter accepts "important") and the Debug Console
    // (which whitelists "important"). A toast alone is too easy to miss.
    sessionState.update((m) => {
      ensureSession(e.sessionId);
      m[e.sessionId].output.push({
        cat: "important",
        text: `[error] ${e.message}\n`,
      });
      return { ...m };
    });
    const { showError } = await import("./toast");
    showError("Debug session error", e.message, e.sessionId);
    return;
  }
  if (e.kind === "state" && e.state) {
    // Terminated sessions used to be auto-removed after 1.5s, but that wiped
    // their output before the user could inspect it (and broke the "Show in
    // Debug Console" toast action, which set activeSessionId to a session
    // that no longer existed). Now we always just update state and let the
    // user dismiss terminated sessions explicitly via the panels.
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
    // Auto-switch panels when stopped: source on right, variables on left
    import("./panels/layout").then(({ setActivePanel }) => {
      setActivePanel("right", "source");
      setActivePanel("left", "variables");
    });
    sessions.update((m) => {
      if (m[e.sessionId]) m[e.sessionId] = { ...m[e.sessionId], state: "stopped" };
      return m;
    });
    // auto-refresh stack
    fetchStack(e.sessionId).catch(() => {});
  }
}));

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
