import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as AnalyticsService from "../../bindings/locafy.dev/desktop/internal/analytics/service";
import type { Event, Trend } from "../../bindings/locafy.dev/desktop/internal/analytics/models";
import * as ApplicationsService from "../../bindings/locafy.dev/desktop/internal/applications/service";
import type { Application } from "../../bindings/locafy.dev/desktop/internal/applications/models";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import type { Project } from "../../bindings/locafy.dev/desktop/internal/projects/models";
import * as StatusService from "../../bindings/locafy.dev/desktop/internal/status/service";
import type { Health } from "../../bindings/locafy.dev/desktop/internal/status/models";
import { AnalyticsScreen } from "./analytics";

vi.mock("../../bindings/locafy.dev/desktop/internal/analytics/service", () => ({
  Trend: vi.fn(),
  Retention: vi.fn(),
  SetRetention: vi.fn(),
  Recent: vi.fn(),
}));

vi.mock("../../bindings/locafy.dev/desktop/internal/applications/service", () => ({
  List: vi.fn(),
  Lifecycle: vi.fn(),
  Lifecycles: vi.fn(),
}));

vi.mock("../../bindings/locafy.dev/desktop/internal/projects/service", () => ({
  List: vi.fn(),
}));

vi.mock("../../bindings/locafy.dev/desktop/internal/status/service", () => ({
  Health: vi.fn(),
}));

const trend = (overrides: Partial<Trend> = {}): Trend => ({
  days: [
    { day: "2026-11-24", count: 0 },
    { day: "2026-11-25", count: 3 },
    { day: "2026-11-26", count: 5 },
  ],
  actions: [
    { action: "Project started", count: 5 },
    { action: "Service added", count: 3 },
  ],
  total: 8,
  retainedDays: 90,
  ...overrides,
});

const project = (overrides: Partial<Project> = {}): Project => ({
  ID: "p1",
  Name: "Etugora",
  RootPath: "C:\\locafy\\projects\\etugora",
  Description: "",
  Kind: "managed",
  Status: "active",
  LifecycleState: "running",
  CreatedAt: "2026-11-14T09:00:00Z",
  UpdatedAt: "2026-11-14T09:00:00Z",
  ...overrides,
});

const application = (overrides: Partial<Application> = {}): Application => ({
  ID: "app-1",
  ProjectID: "p1",
  Name: "web",
  Directory: ".",
  Language: "TypeScript",
  Framework: "Next.js",
  Marker: "package.json",
  StartCommand: "npm run dev",
  StopCommand: "",
  Status: "running",
  ExitCode: null,
  Error: "",
  StartedAt: "",
  Port: 3000,
  PortConflict: "",
  Health: "",
  HealthDetail: "",
  HealthCheckedAt: "",
  RuntimeID: "",
  Source: "detected",
  ScaffoldRecipe: "",
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
  DatabasePath: "C:\\data\\locafy.db",
  DatabaseOK: true,
  StartedAt: "2026-11-14T09:00:00Z",
  UptimeSeconds: 120,
  CPUPercent: 24,
  Memory: { UsedBytes: 8 * 1024 ** 3, TotalBytes: 32 * 1024 ** 3, Percent: 25 },
  Disk: { UsedBytes: 120 * 1024 ** 3, TotalBytes: 500 * 1024 ** 3, Percent: 24 },
  ...overrides,
});

const event = (overrides: Partial<Event> = {}) => ({
  id: "n1",
  scopeId: "p1",
  scope: "project",
  action: "Project started",
  detail: "started web",
  at: new Date().toISOString().replace("T", " ").replace(/\..*$/, ""),
  ...overrides,
});

function renderAnalytics() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  render(
    <QueryClientProvider client={client}>
      <AnalyticsScreen />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.mocked(AnalyticsService.Trend).mockReset();
  vi.mocked(AnalyticsService.Trend).mockResolvedValue(trend());
  vi.mocked(AnalyticsService.Retention).mockReset();
  vi.mocked(AnalyticsService.Retention).mockResolvedValue(90);
  vi.mocked(AnalyticsService.SetRetention).mockReset();
  vi.mocked(AnalyticsService.SetRetention).mockResolvedValue();
  vi.mocked(ProjectsService.List).mockReset();
  vi.mocked(ProjectsService.List).mockResolvedValue([project()]);
  vi.mocked(ApplicationsService.List).mockReset();
  vi.mocked(ApplicationsService.List).mockResolvedValue([application()]);
  vi.mocked(ApplicationsService.Lifecycles).mockReset();
  vi.mocked(ApplicationsService.Lifecycles).mockResolvedValue({ p1: "running" });
  vi.mocked(StatusService.Health).mockReset();
  vi.mocked(StatusService.Health).mockResolvedValue(health());
  vi.mocked(AnalyticsService.Recent).mockReset();
  vi.mocked(AnalyticsService.Recent).mockResolvedValue([event()]);
});

describe("AnalyticsScreen", () => {
  it("keeps the resource, status and timeline sections on live readings", async () => {
    renderAnalytics();

    expect(await screen.findByText("Resource usage — this machine")).toBeInTheDocument();
    expect(await screen.findByText("24")).toBeInTheDocument();
    expect(screen.getAllByText("Machine-wide").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("no trend yet")).toHaveLength(3);

    expect(await screen.findByText("Project status — now")).toBeInTheDocument();
    expect(await screen.findByText("Etugora")).toBeInTheDocument();
    expect(screen.getByRole("table")).toHaveTextContent("Etugora");

    expect(await screen.findByText("Activity timeline — today")).toBeInTheDocument();
    expect((await screen.findAllByText("Project started")).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/— started web/)).toBeInTheDocument();
  });

  it("adds the recorded actions trend and the action mix", async () => {
    renderAnalytics();

    expect(await screen.findByText("Recorded actions — last 30 days")).toBeInTheDocument();
    expect(await screen.findByText("8")).toBeInTheDocument();
    expect(screen.getByText(/actions in the last 30 days/)).toBeInTheDocument();
    expect(screen.getByText("Service added")).toBeInTheDocument();
    expect(AnalyticsService.Trend).toHaveBeenCalledWith(30);
    expect(await screen.findByText(/Updated \d+ s ago/)).toBeInTheDocument();
    expect(screen.getByText(/kept for 90 days/)).toBeInTheDocument();
  });

  it("stores a new retention window", async () => {
    const user = userEvent.setup();
    renderAnalytics();

    await user.selectOptions(await screen.findByLabelText("Keep history for"), "30");

    expect(AnalyticsService.SetRetention).toHaveBeenCalledWith(30);
    await waitFor(() => expect(AnalyticsService.Trend).toHaveBeenCalled());
  });

  it("says nothing has been recorded in an empty window", async () => {
    vi.mocked(AnalyticsService.Trend).mockResolvedValue(
      trend({ days: [{ day: "2026-11-26", count: 0 }], actions: [], total: 0 }),
    );
    vi.mocked(AnalyticsService.Recent).mockResolvedValue([]);

    renderAnalytics();

    expect(await screen.findByText(/Nothing has been recorded in this window yet/)).toBeInTheDocument();
    expect(screen.getByText("No actions recorded in this window.")).toBeInTheDocument();
    expect(screen.getByText("Nothing has been recorded today yet.")).toBeInTheDocument();
  });
});
