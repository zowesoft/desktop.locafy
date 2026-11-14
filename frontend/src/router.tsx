import { createHashRouter } from "react-router-dom";
import App from "@/App";
import { Dashboard } from "@/screens/dashboard";
import { NotFound } from "@/screens/not-found";
import { Placeholder } from "@/screens/placeholder";
import { RouteError } from "@/screens/route-error";
import { SettingsScreen } from "@/screens/settings";

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
      { path: "projects", element: <Placeholder name="Projects" /> },
      { path: "applications", element: <Placeholder name="Applications" /> },
      { path: "services", element: <Placeholder name="Services" /> },
      { path: "runtimes", element: <Placeholder name="Runtimes" /> },
      { path: "downloads", element: <Placeholder name="Downloads" /> },
      { path: "domains", element: <Placeholder name="Domains" /> },
      { path: "analytics", element: <Placeholder name="Analytics" /> },
      { path: "logs", element: <Placeholder name="Logs" /> },
      { path: "terminal", element: <Placeholder name="Terminal" /> },
      { path: "tunnels", element: <Placeholder name="Tunnels" planned /> },
      { path: "extensions", element: <Placeholder name="Extensions" planned /> },
      { path: "help", element: <Placeholder name="Help & Support" /> },
      { path: "settings", element: <SettingsScreen /> },
      { path: "*", element: <NotFound /> },
    ],
  },
]);
