<script lang="ts">
  import SidebarSection from "./SidebarSection.svelte";
  import SessionsSection from "./SessionsSection.svelte";
  import FileTreePanel from "./FileTreePanel.svelte";
  import BreakpointsPanel from "./BreakpointsPanel.svelte";
  import CallStackPanel from "./CallStackPanel.svelte";
  import ThreadsPanel from "./ThreadsPanel.svelte";
  import { globalBreakpoints } from "./store";

  $: breakpointCount = Object.values($globalBreakpoints)
    .reduce((n, arr) => n + (arr?.length ?? 0), 0);
</script>

<aside class="sidebar">
  <SessionsSection />

  <SidebarSection id="filetree" label="Files" flex>
    <FileTreePanel hideHeader />
  </SidebarSection>

  <SidebarSection id="breakpoints" label="Breakpoints" count={breakpointCount > 0 ? breakpointCount : undefined}>
    <BreakpointsPanel hideHeader />
  </SidebarSection>

  <SidebarSection id="callstack" label="Call Stack">
    <CallStackPanel hideHeader />
  </SidebarSection>

  <SidebarSection id="threads" label="Threads">
    <ThreadsPanel hideHeader />
  </SidebarSection>
</aside>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    background: var(--bg-subtle);
    border-right: 1px solid var(--border-subtle);
    padding: 6px 0;
    gap: 2px;
  }
  :global(body.mac) .sidebar {
    background: var(--bg-subtle);
    border-right-color: rgba(0, 0, 0, 0.35);
    box-shadow: inset -1px 0 0 rgba(255, 255, 255, 0.025);
  }
</style>
