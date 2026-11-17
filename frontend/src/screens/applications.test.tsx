import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
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

vi.mock("../../bindings/locafy.dev/desktop/internal/applications/service", () => ({
  List: vi.fn(),
  Detect: vi.fn(),
  Create: vi.fn(),
  Remove: vi.fn(),
  SetDependencies: vi.fn(),
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
  Source: "detected",
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

    await user.click(await screen.findByRole("button", { name: "Add application" }));
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

    await user.click(await screen.findByRole("button", { name: "Add application" }));
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

    await user.click(await screen.findByRole("button", { name: "Add application" }));
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
