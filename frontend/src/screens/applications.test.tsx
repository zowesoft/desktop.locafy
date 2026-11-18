import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { act } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as ApplicationsService from "../../bindings/locafy.dev/desktop/internal/applications/service";
import type {
  Application,
  Detection,
} from "../../bindings/locafy.dev/desktop/internal/applications/models";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import type { Project } from "../../bindings/locafy.dev/desktop/internal/projects/models";
import { projectWorkspaceRoute } from "./project-workspace";

vi.mock("../../bindings/locafy.dev/desktop/internal/projects/service", () => ({
  List: vi.fn(),
}));

const handlers = vi.hoisted(() => new Map<string, (event: { data: unknown }) => void>());

vi.mock("@wailsio/runtime", () => ({
  Events: {
    On: (name: string, callback: (event: { data: unknown }) => void) => {
      handlers.set(name, callback);
      return () => handlers.delete(name);
    },
  },
}));

vi.mock("../../bindings/locafy.dev/desktop/internal/applications/service", () => ({
  List: vi.fn(),
  Detect: vi.fn(),
  Create: vi.fn(),
  Remove: vi.fn(),
  SetDependencies: vi.fn(),
  Start: vi.fn(),
  Stop: vi.fn(),
  Restart: vi.fn(),
  Output: vi.fn(),
}));

const project: Project = {
  ID: "p1",
  Name: "Etugora",
  RootPath: "C:\\locafy\\projects\\etugora",
  Description: "",
  Kind: "managed",
  Status: "active",
  LifecycleState: "stopped",
  CreatedAt: "2026-11-14T09:00:00Z",
  UpdatedAt: "2026-11-14T09:00:00Z",
};

const application = (overrides: Partial<Application> = {}): Application => ({
  ID: "a1",
  ProjectID: "p1",
  Name: "API",
  Directory: "apps/api",
  Language: "PHP",
  Framework: "Laravel",
  Marker: "composer.json",
  StartCommand: "php artisan serve",
  StopCommand: "",
  Status: "stopped",
  ExitCode: null,
  Error: "",
  StartedAt: "",
  Port: null,
  PortConflict: "",
  Source: "detected",
  ScaffoldRecipe: "",
  ScaffoldOptions: "",
  CreatedAt: "2026-11-14T09:00:00Z",
  UpdatedAt: "2026-11-14T09:00:00Z",
  DependsOn: [],
  ...overrides,
});

const detection = (overrides: Partial<Detection> = {}): Detection => ({
  Name: "Storefront",
  Directory: "apps/web",
  Language: "TypeScript",
  Framework: "Next.js",
  Marker: "package.json",
  StartCommand: "npm run dev",
  ...overrides,
});

function renderAt(path: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  });
  const router = createMemoryRouter(
    [{ path: "/projects", element: <p>projects dashboard</p> }, projectWorkspaceRoute],
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
  vi.mocked(ProjectsService.List).mockReset();
  vi.mocked(ProjectsService.List).mockResolvedValue([project]);
  vi.mocked(ApplicationsService.List).mockReset();
  vi.mocked(ApplicationsService.List).mockResolvedValue([]);
  vi.mocked(ApplicationsService.Detect).mockReset();
  vi.mocked(ApplicationsService.Detect).mockResolvedValue([]);
  vi.mocked(ApplicationsService.Create).mockReset();
  vi.mocked(ApplicationsService.SetDependencies).mockReset();
  vi.mocked(ApplicationsService.SetDependencies).mockResolvedValue(undefined);
  vi.mocked(ApplicationsService.Start).mockReset();
  vi.mocked(ApplicationsService.Stop).mockReset();
  vi.mocked(ApplicationsService.Restart).mockReset();
});

describe("ApplicationsSection", () => {
  it("lists each application with its folder, command and dependencies", async () => {
    vi.mocked(ApplicationsService.List).mockResolvedValue([
      application({ ID: "a1", Name: "API", Directory: "apps/api" }),
      application({
        ID: "a2",
        Name: "Storefront",
        Directory: "apps/web",
        Language: "TypeScript",
        Framework: "Next.js",
        StartCommand: "npm run dev",
        DependsOn: ["a1"],
      }),
    ]);

    renderAt("/projects/p1/applications");

    expect(await screen.findByText("Storefront")).toBeInTheDocument();
    const rows = screen.getAllByRole("listitem");
    const storefront = rows.find((row) => within(row).queryByText("Storefront"));

    expect(storefront).toBeDefined();
    expect(within(storefront!).getByText("apps/web")).toBeInTheDocument();
    expect(within(storefront!).getByText("npm run dev")).toBeInTheDocument();
    expect(within(storefront!).getByText("TypeScript")).toBeInTheDocument();
    // Dependencies are shown by name, not by id.
    expect(within(storefront!).getByText("API")).toBeInTheDocument();
  });

  it("says so when the project has no applications", async () => {
    renderAt("/projects/p1/applications");

    expect(await screen.findByText("No applications yet")).toBeInTheDocument();
    expect(ApplicationsService.Detect).not.toHaveBeenCalled();
  });

  it("surfaces a failure to read applications", async () => {
    vi.mocked(ApplicationsService.List).mockRejectedValue(new Error("database is locked"));

    renderAt("/projects/p1/applications");

    expect(await screen.findByRole("alert")).toHaveTextContent("Could not read applications");
  });

  it("offers detection as a proposal and saves nothing until it is confirmed", async () => {
    const user = userEvent.setup();
    vi.mocked(ApplicationsService.Detect).mockResolvedValue([detection()]);
    vi.mocked(ApplicationsService.Create).mockResolvedValue(application());

    renderAt("/projects/p1/applications");

    await user.click(await screen.findByRole("button", { name: "Add existing" }));
    expect(await screen.findByRole("button", { name: "Use the Storefront proposal" })).toBeInTheDocument();
    expect(ApplicationsService.Create).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Add" }));

    await waitFor(() => expect(ApplicationsService.Create).toHaveBeenCalled());
    expect(ApplicationsService.Create).toHaveBeenCalledWith(
      "p1",
      expect.objectContaining({
        Name: "Storefront",
        Directory: "apps/web",
        Language: "TypeScript",
        Framework: "Next.js",
        StartCommand: "npm run dev",
        Marker: "package.json",
        Source: "detected",
      }),
    );
  });

  it("saves the proposal as edited, not as detected", async () => {
    const user = userEvent.setup();
    vi.mocked(ApplicationsService.Detect).mockResolvedValue([detection()]);
    vi.mocked(ApplicationsService.Create).mockResolvedValue(application());

    renderAt("/projects/p1/applications");

    await user.click(await screen.findByRole("button", { name: "Add existing" }));
    const command = await screen.findByRole("textbox", { name: "Start command" });
    await user.clear(command);
    await user.type(command, "npm start");
    await user.click(screen.getByRole("button", { name: "Add" }));

    await waitFor(() =>
      expect(ApplicationsService.Create).toHaveBeenCalledWith(
        "p1",
        expect.objectContaining({ StartCommand: "npm start" }),
      ),
    );
  });

  it("adds an application by hand when nothing is detected", async () => {
    const user = userEvent.setup();
    vi.mocked(ApplicationsService.Create).mockResolvedValue(application());

    renderAt("/projects/p1/applications");

    await user.click(await screen.findByRole("button", { name: "Add existing" }));
    expect(await screen.findByText(/Nothing recognizable was found/)).toBeInTheDocument();

    await user.type(screen.getByRole("textbox", { name: "Application name" }), "Worker");
    await user.type(screen.getByRole("textbox", { name: "Application folder" }), "worker");
    await user.click(screen.getByRole("button", { name: "Add" }));

    await waitFor(() => expect(ApplicationsService.Create).toHaveBeenCalled());
    expect(ApplicationsService.Create).toHaveBeenCalledWith(
      "p1",
      expect.objectContaining({
        Name: "Worker",
        Directory: "worker",
        Source: "manual",
        Marker: "",
      }),
    );
  });

  it("shows a refused cycle instead of saving it", async () => {
    const user = userEvent.setup();
    vi.mocked(ApplicationsService.List).mockResolvedValue([
      application({ ID: "a1", Name: "API" }),
      application({ ID: "a2", Name: "Worker", Directory: "apps/worker" }),
    ]);
    vi.mocked(ApplicationsService.SetDependencies).mockRejectedValue(
      new Error("applications cannot depend on each other in a cycle"),
    );

    renderAt("/projects/p1/applications");

    await user.click(
      await screen.findByRole("button", { name: "Edit dependencies of Worker" }),
    );
    await user.click(await screen.findByRole("checkbox", { name: "Depend on API" }));
    await user.click(screen.getByRole("button", { name: "Save dependencies" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("cycle");
    // The dialog stays open with the choice, and nothing was stored.
    expect(screen.getByRole("button", { name: "Save dependencies" })).toBeInTheDocument();
    expect(ApplicationsService.SetDependencies).toHaveBeenCalledWith("a2", ["a1"]);
  });

  it("starts a stopped application from its saved command", async () => {
    const user = userEvent.setup();
    vi.mocked(ApplicationsService.List).mockResolvedValue([application()]);
    vi.mocked(ApplicationsService.Start).mockResolvedValue(application({ Status: "running" }));

    renderAt("/projects/p1/applications");

    await user.click(await screen.findByRole("button", { name: "Start API" }));

    await waitFor(() => expect(ApplicationsService.Start).toHaveBeenCalledWith("a1"));
  });

  it("stops a running application and shows why a run failed", async () => {
    const user = userEvent.setup();
    vi.mocked(ApplicationsService.List).mockResolvedValue([
      application({ Status: "running" }),
      application({
        ID: "a2",
        Name: "Worker",
        Directory: "apps/worker",
        Status: "error",
        ExitCode: 3,
        Error: "Exited with code 3.",
      }),
    ]);
    vi.mocked(ApplicationsService.Stop).mockResolvedValue(undefined);

    renderAt("/projects/p1/applications");

    expect(await screen.findByText("Exit code 3.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Stop API" }));

    await waitFor(() => expect(ApplicationsService.Stop).toHaveBeenCalledWith("a1"));
  });

  it("shows the port an application listens on", async () => {
    vi.mocked(ApplicationsService.List).mockResolvedValue([
      application({ Status: "running", Port: 8123 }),
    ]);

    renderAt("/projects/p1/applications");

    expect(await screen.findByText("8123")).toBeInTheDocument();
    expect(screen.getByText("Port")).toBeInTheDocument();
  });

  it("flags a port conflict with the process holding it", async () => {
    vi.mocked(ApplicationsService.List).mockResolvedValue([
      application({
        Status: "error",
        Port: 8123,
        PortConflict:
          "Port 8123 is already used by python.exe (PID 777), which Locafy did not start.",
      }),
    ]);

    renderAt("/projects/p1/applications");

    expect(await screen.findByRole("alert")).toHaveTextContent("Port 8123");
    expect(screen.getByRole("alert")).toHaveTextContent("python.exe");
  });

  it("streams a running application's output", async () => {
    vi.mocked(ApplicationsService.List).mockResolvedValue([application({ Status: "running" })]);

    renderAt("/projects/p1/applications");

    await screen.findByText("API");
    act(() => {
      handlers.get("application:output")?.({
        data: { applicationId: "a1", line: "Server listening on 8000" },
      });
    });

    expect(await screen.findByLabelText("Output of API")).toHaveTextContent(
      "Server listening on 8000",
    );
  });

  it("saves the chosen dependencies", async () => {
    const user = userEvent.setup();
    vi.mocked(ApplicationsService.List).mockResolvedValue([
      application({ ID: "a1", Name: "API" }),
      application({ ID: "a2", Name: "Worker", Directory: "apps/worker" }),
    ]);

    renderAt("/projects/p1/applications");

    await user.click(
      await screen.findByRole("button", { name: "Edit dependencies of Worker" }),
    );
    await user.click(await screen.findByRole("checkbox", { name: "Depend on API" }));
    await user.click(screen.getByRole("button", { name: "Save dependencies" }));

    await waitFor(() =>
      expect(ApplicationsService.SetDependencies).toHaveBeenCalledWith("a2", ["a1"]),
    );
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Save dependencies" })).not.toBeInTheDocument(),
    );
  });
});
