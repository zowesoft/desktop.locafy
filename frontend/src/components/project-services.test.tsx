import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as ServicesService from "../../bindings/locafy.dev/desktop/internal/services/service";
import type { Instance, Template } from "../../bindings/locafy.dev/desktop/internal/services/models";
import { ProjectServices } from "./project-services";

vi.mock("../../bindings/locafy.dev/desktop/internal/services/service", () => ({
  List: vi.fn(),
  Catalogue: vi.fn(),
  Create: vi.fn(),
  Start: vi.fn(),
  Stop: vi.fn(),
  Restart: vi.fn(),
  Remove: vi.fn(),
  Output: vi.fn(),
  StopAll: vi.fn(),
}));

const service = (overrides: Partial<Instance> = {}): Instance => ({
  ID: "s1",
  ProjectID: "p1",
  Name: "PostgreSQL",
  Engine: "postgresql",
  Version: "17.6",
  Port: 5433,
  DataDirectory: "C:\\Users\\me\\AppData\\Roaming\\Locafy\\services\\s1\\data",
  DatabaseName: "locafy",
  UserName: "locafy",
  Status: "running",
  Error: "",
  StartedAt: "2026-11-23T10:00:00Z",
  Health: "healthy",
  HealthDetail: "Answering on port 5433.",
  HealthCheckedAt: "2026-11-23T10:00:02Z",
  PID: 4242,
  CreatedAt: "2026-11-23T09:59:00Z",
  UpdatedAt: "2026-11-23T10:00:02Z",
  Connection: {
    Host: "127.0.0.1",
    HostPort: "127.0.0.1:5433",
    Port: 5433,
    Database: "locafy",
    User: "locafy",
    URL: "postgresql://locafy@127.0.0.1:5433/locafy",
    Environment: "DATABASE_URL=postgresql://locafy@127.0.0.1:5433/locafy",
  },
  ...overrides,
});

const template = (overrides: Partial<Template> = {}): Template => ({
  Engine: "postgresql",
  Name: "PostgreSQL",
  Summary: "Relational database, one data directory and port per project.",
  DefaultPort: 5432,
  Version: "17.6",
  Available: true,
  Location: "C:\\Program Files\\PostgreSQL\\17\\bin\\postgres.exe",
  Detail: "",
  ...overrides,
});

function renderServices() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <ProjectServices projectID="p1" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.mocked(ServicesService.List).mockReset();
  vi.mocked(ServicesService.List).mockResolvedValue([]);
  vi.mocked(ServicesService.Catalogue).mockReset();
  vi.mocked(ServicesService.Catalogue).mockResolvedValue([template()]);
  vi.mocked(ServicesService.Create).mockReset();
  vi.mocked(ServicesService.Create).mockResolvedValue(service({ Status: "stopped", Health: "" }));
  vi.mocked(ServicesService.Start).mockReset();
  vi.mocked(ServicesService.Start).mockResolvedValue(service());
  vi.mocked(ServicesService.Stop).mockReset();
  vi.mocked(ServicesService.Stop).mockResolvedValue();
  vi.mocked(ServicesService.Restart).mockReset();
  vi.mocked(ServicesService.Restart).mockResolvedValue(service());
  vi.mocked(ServicesService.Remove).mockReset();
  vi.mocked(ServicesService.Remove).mockResolvedValue({
    Name: "PostgreSQL",
    DataDirectory: "C:\\Users\\me\\AppData\\Roaming\\Locafy\\services\\s1\\data",
  });
});

describe("ProjectServices", () => {
  it("lists a service with its engine, port and live health", async () => {
    vi.mocked(ServicesService.List).mockResolvedValue([service()]);

    renderServices();

    expect(await screen.findByText("PostgreSQL")).toBeInTheDocument();
    expect(screen.getByText("PostgreSQL 17.6")).toBeInTheDocument();
    expect(screen.getByText("127.0.0.1:5433")).toBeInTheDocument();
    expect(screen.getByText("Running · Answering")).toBeInTheDocument();
  });

  it("offers the connection details the application needs", async () => {
    const user = userEvent.setup();
    vi.mocked(ServicesService.List).mockResolvedValue([service()]);

    renderServices();

    await user.click(
      await screen.findByRole("button", { name: "Show connection details for PostgreSQL" }),
    );

    expect(
      screen.getByText("postgresql://locafy@127.0.0.1:5433/locafy"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("DATABASE_URL=postgresql://locafy@127.0.0.1:5433/locafy"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("C:\\Users\\me\\AppData\\Roaming\\Locafy\\services\\s1\\data"),
    ).toBeInTheDocument();
  });

  it("attaches a service from the catalogue under the name the operator gives", async () => {
    const user = userEvent.setup();

    renderServices();

    await user.click(await screen.findByRole("button", { name: "Add service" }));
    const dialog = within(screen.getByRole("dialog"));
    expect(dialog.getByText(/Relational database/)).toBeInTheDocument();

    await user.type(dialog.getByLabelText("Name"), "Analytics");
    await user.click(dialog.getByRole("button", { name: "Add service" }));

    expect(ServicesService.Create).toHaveBeenCalledWith("p1", {
      Engine: "postgresql",
      Name: "Analytics",
    });
  });

  it("will not attach an engine that is not installed", async () => {
    const user = userEvent.setup();
    vi.mocked(ServicesService.Catalogue).mockResolvedValue([
      template({ Available: false, Version: "", Detail: "PostgreSQL is not installed on this machine." }),
    ]);

    renderServices();

    await user.click(await screen.findByRole("button", { name: "Add service" }));
    const dialog = within(screen.getByRole("dialog"));

    expect(dialog.getByText(/not installed on this machine/)).toBeInTheDocument();
    expect(dialog.getByRole("button", { name: "Add service" })).toBeDisabled();
    expect(dialog.getByText("Not installed")).toBeInTheDocument();
  });

  it("starts, stops and restarts from the row", async () => {
    const user = userEvent.setup();
    vi.mocked(ServicesService.List).mockResolvedValue([service({ Status: "stopped", Health: "" })]);

    renderServices();

    await user.click(await screen.findByRole("button", { name: "Start PostgreSQL" }));

    expect(ServicesService.Start).toHaveBeenCalledWith("s1");
  });

  it("stops and restarts a running service", async () => {
    const user = userEvent.setup();
    vi.mocked(ServicesService.List).mockResolvedValue([service()]);

    renderServices();

    await user.click(await screen.findByRole("button", { name: "Stop PostgreSQL" }));
    expect(ServicesService.Stop).toHaveBeenCalledWith("s1");

    await user.click(screen.getByRole("button", { name: "Restart PostgreSQL" }));
    expect(ServicesService.Restart).toHaveBeenCalledWith("s1");
  });

  it("keeps the data directory when the service is removed, and says so first", async () => {
    const user = userEvent.setup();
    vi.mocked(ServicesService.List).mockResolvedValue([service()]);

    renderServices();

    await user.click(await screen.findByRole("button", { name: "Remove PostgreSQL" }));

    const dialog = within(screen.getByRole("alertdialog"));
    expect(dialog.getByText(/stays on disk/)).toBeInTheDocument();
    expect(
      dialog.getByText("C:\\Users\\me\\AppData\\Roaming\\Locafy\\services\\s1\\data"),
    ).toBeInTheDocument();
    expect(ServicesService.Remove).not.toHaveBeenCalled();

    await user.click(dialog.getByRole("button", { name: "Remove service" }));

    expect(ServicesService.Remove).toHaveBeenCalledWith("s1");
  });

  it("reports a refused start instead of pretending it ran", async () => {
    const user = userEvent.setup();
    vi.mocked(ServicesService.List).mockResolvedValue([service({ Status: "stopped", Health: "" })]);
    vi.mocked(ServicesService.Start).mockRejectedValue(
      new Error("the service's port is already in use: port 5433 is held by another process"),
    );

    renderServices();

    await user.click(await screen.findByRole("button", { name: "Start PostgreSQL" }));

    expect(await screen.findByText(/port 5433 is held by another process/)).toBeInTheDocument();
  });

  it("shows the empty state before anything is attached", async () => {
    renderServices();

    expect(await screen.findByText("No services yet")).toBeInTheDocument();
  });
});
