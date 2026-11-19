import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as ApplicationsService from "../../bindings/locafy.dev/desktop/internal/applications/service";
import type { Application } from "../../bindings/locafy.dev/desktop/internal/applications/models";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import type { Project } from "../../bindings/locafy.dev/desktop/internal/projects/models";
import * as StatusService from "../../bindings/locafy.dev/desktop/internal/status/service";
import type { Health } from "../../bindings/locafy.dev/desktop/internal/status/models";
import { projectWorkspaceRoute } from "./project-workspace";

vi.mock("../../bindings/locafy.dev/desktop/internal/projects/service", () => ({
  List: vi.fn(),
  Activity: vi.fn(),
  OpenFolder: vi.fn(),
  OverrideStatus: vi.fn(),
  SetOverrideFile: vi.fn(),
}));

vi.mock("../../bindings/locafy.dev/desktop/internal/applications/service", () => ({
  List: vi.fn(),
  Lifecycle: vi.fn(),
  ProjectUsage: vi.fn(),
  StartProject: vi.fn(),
  StopProject: vi.fn(),
  RestartProject: vi.fn(),
}));

vi.mock("../../bindings/locafy.dev/desktop/internal/status/service", () => ({
  Health: vi.fn(),
}));

const project = (overrides: Partial<Project> = {}): Project => ({
  ID: "p1",
  Name: "Etugora",
  RootPath: "C:\\locafy\\projects\\etugora",
  Description: "",
  Kind: "managed",
  Status: "active",
  LifecycleState: "stopped",
  CreatedAt: "2026-11-14T09:00:00Z",
  UpdatedAt: "2026-11-14T09:00:00Z",
  ...overrides,
});

const application = (overrides: Partial<Application> = {}): Application => ({
  ID: "app-1",
  ProjectID: "p1",
  Name: "api",
  Directory: ".",
  Language: "PHP",
  Framework: "Laravel",
  Marker: "artisan",
  StartCommand: "php artisan serve",
  StopCommand: "",
  Status: "stopped",
  ExitCode: null,
  Error: "",
  StartedAt: "",
  Port: null,
  PortConflict: "",
  Health: "",
  HealthDetail: "",
  HealthCheckedAt: "",
  RuntimeID: "",
  Source: "detected",
  ScaffoldRecipe: "laravel",
  ScaffoldOptions: "{}",
  CreatedAt: "2026-11-14T09:00:00Z",
  UpdatedAt: "2026-11-14T09:00:00Z",
  DependsOn: [],
  ...overrides,
});

const health = (overrides: Partial<Health> = {}): Health => ({
  Name: "locafy",
  Healthy: true,
  Summary: "All systems healthy",
  Hostname: "desktop",
  Platform: "windows",
  Architecture: "amd64",
  DatabasePath: "C:\\Users\\me\\AppData\\Roaming\\Locafy\\data\\locafy.db",
  DatabaseOK: true,
  StartedAt: "2026-11-14T09:00:00Z",
  UptimeSeconds: 120,
  CPUPercent: 24,
  Memory: { UsedBytes: 8 * 1024 ** 3, TotalBytes: 32 * 1024 ** 3, Percent: 25 },
  Disk: { UsedBytes: 120 * 1024 ** 3, TotalBytes: 500 * 1024 ** 3, Percent: 24 },
  ...overrides,
});

function renderAt(path: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const router = createMemoryRouter(
    [
      { path: "/projects", element: <p>projects dashboard</p> },
      projectWorkspaceRoute,
    ],
    { initialEntries: [path] },
  );

  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );

  return router;
}

beforeEach(() => {
  vi.mocked(ProjectsService.Activity).mockReset();
  vi.mocked(ProjectsService.Activity).mockResolvedValue([]);
  vi.mocked(ProjectsService.OpenFolder).mockReset();
  vi.mocked(ProjectsService.OpenFolder).mockResolvedValue();
  vi.mocked(ApplicationsService.List).mockReset();
  vi.mocked(ApplicationsService.List).mockResolvedValue([]);
  vi.mocked(ApplicationsService.Lifecycle).mockReset();
  vi.mocked(ApplicationsService.Lifecycle).mockResolvedValue("stopped");
  vi.mocked(ApplicationsService.ProjectUsage).mockReset();
  vi.mocked(ApplicationsService.ProjectUsage).mockResolvedValue({
    cpuPercent: 0,
    memoryBytes: 0,
    runningApplications: 0,
  });
  vi.mocked(ApplicationsService.StartProject).mockReset();
  vi.mocked(ApplicationsService.StartProject).mockResolvedValue({
    projectID: "p1", started: [], alreadyRunning: [], skipped: [], failed: [], stopped: [],
  });
  vi.mocked(ApplicationsService.StopProject).mockReset();
  vi.mocked(ApplicationsService.StopProject).mockResolvedValue({
    projectID: "p1", started: [], alreadyRunning: [], skipped: [], failed: [], stopped: [],
  });
  vi.mocked(ApplicationsService.RestartProject).mockReset();
  vi.mocked(ApplicationsService.RestartProject).mockResolvedValue({
    projectID: "p1", started: [], alreadyRunning: [], skipped: [], failed: [], stopped: [],
  });
  vi.mocked(StatusService.Health).mockReset();
  vi.mocked(StatusService.Health).mockResolvedValue(health());
  vi.mocked(ProjectsService.OverrideStatus).mockReset();
  vi.mocked(ProjectsService.SetOverrideFile).mockReset();
  vi.mocked(ProjectsService.OverrideStatus).mockResolvedValue({
    Path: "C:\\locafy\\projects\\etugora\\locafy.project.json",
    Present: false,
  });
  vi.mocked(ProjectsService.List).mockReset();
  vi.mocked(ProjectsService.List).mockResolvedValue([
    project(),
    project({ ID: "p2", Name: "checkout", Kind: "linked", RootPath: "D:\\work\\checkout" }),
  ]);
});

describe("ProjectWorkspace", () => {
  it("names the project, its state and its folder", async () => {
    renderAt("/projects/p1");

    expect(await screen.findByRole("heading", { level: 1, name: "Etugora" })).toBeInTheDocument();
    expect(screen.getAllByText("Stopped").length).toBeGreaterThan(0);
    expect(screen.getAllByText("C:\\locafy\\projects\\etugora").length).toBeGreaterThan(0);
  });

  it("opens on the overview with the project's details", async () => {
    renderAt("/projects/p1");

    expect(await screen.findByText("Project status")).toBeInTheDocument();
    expect(screen.getByText("Project details")).toBeInTheDocument();
    expect(screen.getByText(/Locafy created this folder/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Overview" })).toHaveAttribute("aria-current", "page");
  });

  it("summarizes the project's applications and machine state", async () => {
    vi.mocked(ApplicationsService.List).mockResolvedValue([
      application(),
      application({ ID: "app-2", Name: "web", Framework: "Vite", Directory: "apps/web" }),
    ]);
    vi.mocked(ApplicationsService.ProjectUsage).mockResolvedValue({
      cpuPercent: 24,
      memoryBytes: 48 * 1024 ** 3,
      runningApplications: 1,
    });

    renderAt("/projects/p1");

    expect(await screen.findByRole("group", { name: "Applications: 2" })).toBeInTheDocument();
    expect(screen.getByText("2 with a start command")).toBeInTheDocument();
    expect(await screen.findByText("api")).toBeInTheDocument();
    expect(screen.getByText("apps/web")).toBeInTheDocument();
    expect(screen.getByText("All systems healthy")).toBeInTheDocument();
    // The CPU meter is the project's managed usage, not the machine's.
    expect(screen.getByText("24%")).toBeInTheDocument();
    expect(screen.getByText("Managed CPU")).toBeInTheDocument();
    expect(screen.getByText("1 managed application running")).toBeInTheDocument();
    expect(screen.getByText("Machine memory")).toBeInTheDocument();
    expect(screen.getByText("Machine disk")).toBeInTheDocument();
  });

  it("marks services, domains and tunnels as planned instead of inventing them", async () => {
    renderAt("/projects/p1");

    expect(await screen.findByRole("group", { name: "Services: 0" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Domains: 0" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Tunnels: 0" })).toBeInTheDocument();
    expect(screen.getAllByText("Planned for a later phase").length).toBeGreaterThanOrEqual(3);
  });

  it("opens the project folder on request", async () => {
    const user = userEvent.setup();
    renderAt("/projects/p1");

    await user.click(await screen.findByRole("button", { name: /Open folder/ }));

    expect(ProjectsService.OpenFolder).toHaveBeenCalledWith("p1");
  });

  it("lists the project's recent activity", async () => {
    vi.mocked(ProjectsService.Activity).mockResolvedValue([
      {
        ID: "a1",
        Scope: "project" as never,
        ScopeID: "p1",
        Action: "Application added",
        Detail: "api",
        CreatedAt: "2026-11-14T09:00:00Z",
      },
    ]);

    renderAt("/projects/p1");

    expect(await screen.findByText("Application added")).toBeInTheDocument();
    expect(screen.getByText(/— api/)).toBeInTheDocument();
  });

  it("describes a linked project as one Locafy never writes into", async () => {
    renderAt("/projects/p2");

    expect(await screen.findByText(/Locafy never writes into it/)).toBeInTheDocument();
  });

  it("holds every project-scoped section as a tab", async () => {
    renderAt("/projects/p1");

    const tabs = within(await screen.findByRole("navigation", { name: "Project sections" }));

    expect(tabs.getAllByRole("link").map((link) => link.textContent)).toEqual([
      "Overview",
      "Applications",
      "Services",
      "Domains",
      "Environment",
      "Logs",
      "Terminal",
      "Processes",
      "Settings",
    ]);
  });

  it("shows the tunnels tab as planned and not selectable", async () => {
    renderAt("/projects/p1");

    const tabs = await screen.findByRole("navigation", { name: "Project sections" });
    const planned = tabs.querySelector('[aria-disabled="true"]');

    expect(planned).toHaveTextContent("TunnelsPlanned");
    expect(within(tabs).queryByRole("link", { name: /Tunnels/ })).toBeNull();
  });

  it("switches sections without leaving the project", async () => {
    const user = userEvent.setup();
    const router = renderAt("/projects/p1");

    await user.click(await screen.findByRole("link", { name: "Services" }));

    expect(router.state.location.pathname).toBe("/projects/p1/services");
    expect(screen.getByRole("heading", { level: 1, name: "Etugora" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Services" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Services" })).toHaveAttribute("aria-current", "page");
  });

  it("reports an unknown project and offers the way back", async () => {
    const user = userEvent.setup();
    renderAt("/projects/missing");

    expect(await screen.findByText("Project not found")).toBeInTheDocument();
    expect(screen.getByText(/folder and files are untouched/i)).toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "Projects" }));

    expect(await screen.findByText("projects dashboard")).toBeInTheDocument();
  });

  it("keeps the settings file absent until an override is turned on", async () => {
    const user = userEvent.setup();
    vi.mocked(ProjectsService.SetOverrideFile).mockResolvedValue({
      Path: "C:\\locafy\\projects\\etugora\\locafy.project.json",
      Present: true,
    });

    renderAt("/projects/p1/settings");

    const toggle = await screen.findByRole("switch", { name: /Override settings/ });
    expect(toggle).not.toBeChecked();
    expect(screen.queryByText("C:\\locafy\\projects\\etugora\\locafy.project.json")).toBeNull();

    await user.click(toggle);

    expect(ProjectsService.SetOverrideFile).toHaveBeenCalledWith("p1", true);
    expect(
      await screen.findByText("C:\\locafy\\projects\\etugora\\locafy.project.json"),
    ).toBeInTheDocument();
  });

  it("surfaces a failure to read projects", async () => {
    vi.mocked(ProjectsService.List).mockRejectedValue(new Error("database is locked"));

    renderAt("/projects/p1");

    expect(await screen.findByRole("alert")).toHaveTextContent("database is locked");
  });

  it("starts, stops and restarts the project from the overview", async () => {
    const user = userEvent.setup();
    renderAt("/projects/p1");

    await user.click(await screen.findByRole("button", { name: /Start project/i }));
    expect(ApplicationsService.StartProject).toHaveBeenCalledWith("p1");
    await user.click(await screen.findByRole("button", { name: /Stop project/i }));
    expect(ApplicationsService.StopProject).toHaveBeenCalledWith("p1");
    await user.click(await screen.findByRole("button", { name: /^Restart/i }));
    expect(ApplicationsService.RestartProject).toHaveBeenCalledWith("p1");
  });

  it("shows the derived lifecycle state instead of the stored word", async () => {
    vi.mocked(ApplicationsService.Lifecycle).mockResolvedValue("partial");

    renderAt("/projects/p1");

    expect((await screen.findAllByText("Partially running")).length).toBeGreaterThan(0);
    expect(screen.queryByText("Stopped")).not.toBeInTheDocument();
  });

  it("reports a refused project action inside the overview", async () => {
    vi.mocked(ApplicationsService.StartProject).mockRejectedValue(
      new Error("a project start or stop is already in progress: the project is starting"),
    );
    const user = userEvent.setup();
    renderAt("/projects/p1");

    await user.click(await screen.findByRole("button", { name: /Start project/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent("The project action failed:");
    expect(screen.getByRole("alert")).toHaveTextContent("already in progress");
  });
});
