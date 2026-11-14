import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { AppShell } from "@/components/app-shell";
import type { NavId } from "@/components/app-shell";

/** The route each navigation entry points at. */
const navPaths: Record<NavId, string> = {
  overview: "/",
  projects: "/projects",
  applications: "/applications",
  services: "/services",
  runtimes: "/runtimes",
  downloads: "/downloads",
  domains: "/domains",
  analytics: "/analytics",
  tunnels: "/tunnels",
  logs: "/logs",
  terminal: "/terminal",
  extensions: "/extensions",
  help: "/help",
  settings: "/settings",
};

const searchPlaceholders: Record<NavId, string> = {
  overview: "Search projects, apps…",
  projects: "Search projects…",
  applications: "Search applications…",
  services: "Search services…",
  runtimes: "Search runtimes…",
  downloads: "Search downloads…",
  domains: "Search domains…",
  analytics: "Search analytics…",
  tunnels: "Search tunnels…",
  logs: "Search logs…",
  terminal: "Search terminal history…",
  extensions: "Search extensions…",
  help: "Search help…",
  settings: "Search settings…",
};

function navForPath(pathname: string): NavId {
  const match = (Object.entries(navPaths) as [NavId, string][]).find(
    ([, path]) => path === pathname,
  );
  return match?.[0] ?? "overview";
}

/**
 * Route layout: keeps the shell mounted across screens and derives the active
 * navigation entry from the current location, so the sidebar and the content
 * cannot disagree.
 */
export default function App() {
  const location = useLocation();
  const navigate = useNavigate();
  const view = navForPath(location.pathname);

  return (
    <AppShell
      active={view}
      onNavigate={(id) => navigate(navPaths[id])}
      searchPlaceholder={searchPlaceholders[view]}
    >
      <Outlet />
    </AppShell>
  );
}
