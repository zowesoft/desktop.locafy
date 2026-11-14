import { useState } from "react";
import { AppShell } from "@/components/app-shell";
import type { NavId } from "@/components/app-shell";
import { Dashboard } from "@/screens/dashboard";
import { SettingsScreen } from "@/screens/settings";

const searchPlaceholders: Record<NavId, string> = {
  dashboard: "Search projects, apps…",
  projects: "Search projects…",
  tunnels: "Search tunnels…",
  analytics: "Search analytics…",
  settings: "Search settings…",
  extensions: "Search extensions…",
  help: "Search help…",
};

function App() {
  const [view, setView] = useState<NavId>("dashboard");

  return (
    <AppShell
      active={view}
      onNavigate={setView}
      searchPlaceholder={searchPlaceholders[view]}
      context={
        view === "settings" ? (
          <p className="text-sm text-muted-foreground">Settings</p>
        ) : null
      }
    >
      {view === "dashboard" ? (
        <Dashboard />
      ) : view === "settings" ? (
        <SettingsScreen />
      ) : (
        <div className="mx-auto max-w-md px-6 py-24 text-center">
          <p className="text-sm text-muted-foreground">This screen is not built yet.</p>
        </div>
      )}
    </AppShell>
  );
}

export default App;
