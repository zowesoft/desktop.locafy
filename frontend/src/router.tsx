import { createHashRouter } from "react-router-dom";
import App from "@/App";
import { AnalyticsScreen } from "@/screens/analytics";
import { Dashboard } from "@/screens/dashboard";
import { DownloadsScreen } from "@/screens/downloads";
import { HelpScreen } from "@/screens/help";
import { LogsScreen } from "@/screens/logs";
import { NotFound } from "@/screens/not-found";
import { Placeholder } from "@/screens/placeholder";
import { projectWorkspaceRoute } from "@/screens/project-workspace";
import { ProjectsScreen } from "@/screens/projects";
import { RouteError } from "@/screens/route-error";
import { SettingsScreen } from "@/screens/settings";
import { TerminalScreen } from "@/screens/terminal";

/*
 * Hash routing because the app is loaded from a file-style custom scheme in
 * the desktop webview, where history API paths have no server to resolve.
 */
export const router = createHashRouter([
  {
    path: "/",
    element: <App />,
    // Catches failures from any nested route, including the layout itself.
    errorElement: <RouteError />,
    children: [
      { index: true, element: <Dashboard /> },
      { path: "projects", element: <ProjectsScreen /> },
      projectWorkspaceRoute,
      { path: "downloads", element: <DownloadsScreen /> },
      { path: "analytics", element: <AnalyticsScreen /> },
      { path: "logs", element: <LogsScreen /> },
      { path: "terminal", element: <TerminalScreen /> },
      { path: "tunnels", element: <Placeholder name="Tunnels" planned /> },
      { path: "extensions", element: <Placeholder name="Extensions" planned /> },
      { path: "help", element: <HelpScreen /> },
      { path: "settings", element: <SettingsScreen /> },
      { path: "*", element: <NotFound /> },
    ],
  },
]);
