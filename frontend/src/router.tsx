import { createHashRouter } from "react-router-dom";
import App from "@/App";
import { Dashboard } from "@/screens/dashboard";
import { Placeholder } from "@/screens/placeholder";
import { SettingsScreen } from "@/screens/settings";

/*
 * Hash routing because the app is loaded from a file-style custom scheme in
 * the desktop webview, where history API paths have no server to resolve.
 */
export const router = createHashRouter([
  {
    path: "/",
    element: <App />,
    children: [
      { index: true, element: <Dashboard /> },
      { path: "projects", element: <Placeholder name="Projects" /> },
      { path: "tunnels", element: <Placeholder name="Tunnels" /> },
      { path: "analytics", element: <Placeholder name="Analytics" /> },
      { path: "settings", element: <SettingsScreen /> },
      { path: "extensions", element: <Placeholder name="Extensions" /> },
      { path: "help", element: <Placeholder name="Help & Support" /> },
      { path: "*", element: <Placeholder name="Not found" /> },
    ],
  },
]);
