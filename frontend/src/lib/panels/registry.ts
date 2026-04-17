import type { SvelteComponent } from "svelte";
import BreakpointsPanel from "../BreakpointsPanel.svelte";
import TerminalPanel from "../TerminalPanel.svelte";
import ConsolePanel from "../ConsolePanel.svelte";
import VariablesPanel from "../VariablesPanel.svelte";
import CallStackPanel from "../CallStackPanel.svelte";
import ThreadsPanel from "../ThreadsPanel.svelte";
import ResourcesPanel from "../ResourcesPanel.svelte";
import SourcePanel from "../SourcePanel.svelte";

export type DockId = "left" | "right";

export type Panel = {
  id: string;
  title: string;
  icon: string;
  defaultDock: DockId;
  component: typeof SvelteComponent<any>;
};

export const PANELS: Panel[] = [
  {
    id: "breakpoints",
    title: "Breakpoints",
    icon: "solar:record-circle-bold",
    defaultDock: "left",
    component: BreakpointsPanel as any,
  },
  {
    id: "callstack",
    title: "Call Stack",
    icon: "solar:layers-bold",
    defaultDock: "left",
    component: CallStackPanel as any,
  },
  {
    id: "threads",
    title: "Threads",
    icon: "solar:widget-bold",
    defaultDock: "left",
    component: ThreadsPanel as any,
  },
  {
    id: "variables",
    title: "Variables",
    icon: "solar:database-bold",
    defaultDock: "left",
    component: VariablesPanel as any,
  },
  {
    id: "source",
    title: "Source",
    icon: "solar:code-bold",
    defaultDock: "right",
    component: SourcePanel as any,
  },
  {
    id: "terminal",
    title: "Terminal",
    icon: "solar:code-square-bold",
    defaultDock: "right",
    component: TerminalPanel as any,
  },
  {
    id: "console",
    title: "Debug Console",
    icon: "solar:terminal-bold",
    defaultDock: "right",
    component: ConsolePanel as any,
  },
  {
    id: "resources",
    title: "Resources",
    icon: "solar:chart-2-bold",
    defaultDock: "left",
    component: ResourcesPanel as any,
  },
];

export function panelById(id: string): Panel | undefined {
  return PANELS.find((p) => p.id === id);
}
