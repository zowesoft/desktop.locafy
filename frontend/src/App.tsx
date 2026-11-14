import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { AppShell } from "@/components/app-shell";
import type { NavId } from "@/components/app-shell";

/** The route each navigation entry points at. */
const navPaths: Record<NavId, string> = {
  dashboard: "/",
  projects: "/projects",
  tunnels: "/tunnels",
  analytics: "/analytics",
  settings: "/settings",
  extensions: "/extensions",
  help: "/help",
};

const searchPlaceholders: Record<NavId, string> = {
  dashboard: "Search projects, apps…",
  projects: "Search projects…",
  tunnels: "Search tunnels…",
  analytics: "Search analytics…",
  settings: "Search settings…",
  extensions: "Search extensions…",
  help: "Search help…",
};

function navForPath(pathname: string): NavId {
  const match = (Object.entries(navPaths) as [NavId, string][]).find(
    ([, path]) => path === pathname,
  );
  return match?.[0] ?? "dashboard";
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
