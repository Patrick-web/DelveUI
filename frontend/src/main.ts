import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-sans/600.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "./lib/theme.css";
import { initTheme } from "./lib/theme-engine";
import { loadSettings } from "./lib/settings-store";
import { applyPanelSettings } from "./lib/panels/layout";
import App from "./App.svelte";

initTheme();
loadSettings().then(() => applyPanelSettings());

const app = new App({
  target: document.getElementById("app")!,
});

export default app;
