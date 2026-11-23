import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as ProjectsService from "../bindings/locafy.dev/desktop/internal/projects/service";
import App from "./App";

vi.mock("../bindings/locafy.dev/desktop/internal/status/service", () => ({
  Health: vi.fn().mockResolvedValue({
    Name: "Locafy",
    Healthy: true,
    Summary: "All systems operational",
    Hostname: "studio",
    Platform: "windows",
    Architecture: "amd64",
    DatabasePath: "locafy.db",
    DatabaseOK: true,
    StartedAt: "2026-11-15T09:00:00Z",
    UptimeSeconds: 60,
    CPUPercent: 10,
    Memory: { UsedBytes: 1, TotalBytes: 2, Percent: 50 },
    Disk: { UsedBytes: 1, TotalBytes: 2, Percent: 50 },
  }),
  Notifications: vi.fn().mockResolvedValue([]),
}));

vi.mock("../bindings/locafy.dev/desktop/internal/projects/service", () => ({
  List: vi.fn(),
}));

vi.mock("../bindings/locafy.dev/desktop/internal/services/service", () => ({
  List: vi.fn().mockResolvedValue([]),
  Catalogue: vi.fn().mockResolvedValue([]),
}));

function renderAt(path: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const router = createMemoryRouter(
    [
      {
        path: "/",
        element: <App />,
        children: [
          { index: true, element: <p>overview</p> },
          { path: "projects", element: <p>projects dashboard</p> },
          { path: "projects/:projectId/*", element: <p>workspace</p> },
        ],
      },
    ],
    { initialEntries: [path] },
  );

  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.mocked(ProjectsService.List).mockReset();
  vi.mocked(ProjectsService.List).mockResolvedValue([
    {
      ID: "p1",
      Name: "Etugora",
      RootPath: "C:\\locafy\\projects\\etugora",
      Description: "",
      Kind: "managed",
      Status: "active",
      LifecycleState: "stopped",
      CreatedAt: "2026-11-14T09:00:00Z",
      UpdatedAt: "2026-11-14T09:00:00Z",
    },
  ]);
});

describe("App", () => {
  it("puts the project switcher in the top bar inside a workspace", async () => {
    renderAt("/projects/p1/services");

    expect(
      await screen.findByRole(
        "button",
        { name: "Project Etugora, switch project" },
        { timeout: 5000 },
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "All projects" })).toBeInTheDocument();
  });

  it("keeps Projects selected in the sidebar while a workspace is open", async () => {
    renderAt("/projects/p1");

    await screen.findByRole("button", { name: /switch project/ });

    expect(screen.getByRole("button", { name: "Projects" })).toHaveAttribute("aria-current", "page");
  });

  it.each([["/"], ["/projects"]])("shows no project switcher on %s", async (path) => {
    renderAt(path);

    await screen.findByRole("navigation", { name: "Main" });

    expect(screen.queryByRole("button", { name: /switch project/ })).toBeNull();
    expect(screen.queryByRole("link", { name: "All projects" })).toBeNull();
  });
});
