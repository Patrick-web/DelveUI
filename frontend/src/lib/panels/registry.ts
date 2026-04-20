import type { SvelteComponent } from "svelte";
import FileTreePanel from "../FileTreePanel.svelte";
import BreakpointsPanel from "../BreakpointsPanel.svelte";
import TerminalPanel from "../TerminalPanel.svelte";
import ConsolePanel from "../ConsolePanel.svelte";
import VariablesPanel from "../VariablesPanel.svelte";
import CallStackPanel from "../CallStackPanel.svelte";
import ThreadsPanel from "../ThreadsPanel.svelte";
import ResourcesPanel from "../ResourcesPanel.svelte";
import SourcePanel from "../SourcePanel.svelte";
import WatchPanel from "../WatchPanel.svelte";

export type Area = "sidebar" | "inspector" | "center";

export type Panel = {
  id: string;
  title: string;
  icon: string;
  area: Area;
  component: typeof SvelteComponent<any>;
};

export const PANELS: Panel[] = [
  // sidebar tabs
  { id: "filetree",    title: "Files",         icon: "solar:folder-bold",        area: "sidebar",   component: FileTreePanel as any },
  { id: "breakpoints", title: "Breakpoints",   icon: "solar:record-circle-bold", area: "sidebar",   component: BreakpointsPanel as any },
  // inspector tabs
  { id: "variables",   title: "Variables",     icon: "solar:database-bold",      area: "inspector", component: VariablesPanel as any },
  { id: "watch",       title: "Watch",         icon: "solar:eye-bold",           area: "inspector", component: WatchPanel as any },
  { id: "callstack",   title: "Call Stack",    icon: "solar:layers-bold",        area: "inspector", component: CallStackPanel as any },
  { id: "threads",     title: "Threads",       icon: "solar:widget-bold",        area: "inspector", component: ThreadsPanel as any },
  { id: "resources",   title: "Resources",     icon: "solar:chart-2-bold",       area: "inspector", component: ResourcesPanel as any },
  // center tabs
  { id: "source",      title: "Source",        icon: "solar:code-bold",          area: "center",    component: SourcePanel as any },
  { id: "terminal",    title: "Terminal",      icon: "solar:code-square-bold",   area: "center",    component: TerminalPanel as any },
  { id: "console",     title: "Debug Console", icon: "solar:terminal-bold",      area: "center",    component: ConsolePanel as any },
];

export function panelById(id: string): Panel | undefined {
  return PANELS.find((p) => p.id === id);
}

export function panelsInArea(area: Area): Panel[] {
  return PANELS.filter((p) => p.area === area);
}
