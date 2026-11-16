import { Outlet, useLocation, useMatch, useNavigate } from "react-router-dom";
import { AppShell } from "@/components/app-shell";
import type { NavId } from "@/components/app-shell";
import { ProjectSwitcher } from "@/components/project-switcher";
import { EngineStatus, Notifications } from "@/components/top-bar-status";

/** The route each navigation entry points at. */
const navPaths: Record<NavId, string> = {
  overview: "/",
  projects: "/projects",
  runtimes: "/runtimes",
  downloads: "/downloads",
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
  runtimes: "Search runtimes…",
  downloads: "Search downloads…",
  analytics: "Search analytics…",
  tunnels: "Search tunnels…",
  logs: "Search logs…",
  terminal: "Search terminal history…",
  extensions: "Search extensions…",
  help: "Search help…",
  settings: "Search settings…",
};

/** A project workspace lives below `/projects`, so it keeps that entry selected. */
function navForPath(pathname: string): NavId {
  const match = (Object.entries(navPaths) as [NavId, string][]).find(
    ([, path]) => path === pathname || (path !== "/" && pathname.startsWith(`${path}/`)),
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
  const workspace = useMatch("/projects/:projectId/*");

  return (
    <AppShell
      active={view}
      onNavigate={(id) => navigate(navPaths[id])}
      searchPlaceholder={searchPlaceholders[view]}
      context={
        workspace?.params.projectId ? (
          <ProjectSwitcher projectId={workspace.params.projectId} />
        ) : undefined
      }
      topbarActions={
        <>
          <EngineStatus />
          <Notifications />
        </>
      }
    >
      <Outlet />
    </AppShell>
  );
}
