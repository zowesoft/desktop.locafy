import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider, useLocation, useParams } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import type { Project } from "../../bindings/locafy.dev/desktop/internal/projects/models";
import { ProjectSwitcher } from "./project-switcher";

vi.mock("../../bindings/locafy.dev/desktop/internal/projects/service", () => ({
  List: vi.fn(),
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

function Harness() {
  const { projectId } = useParams();
  const location = useLocation();

  return (
    <>
      <ProjectSwitcher projectId={projectId ?? ""} />
      <p data-testid="location">{location.pathname}</p>
    </>
  );
}

function renderAt(path: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const router = createMemoryRouter(
    [
      { path: "/projects", element: <p>projects dashboard</p> },
      { path: "/projects/:projectId/*", element: <Harness /> },
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
  vi.mocked(ProjectsService.List).mockReset();
  vi.mocked(ProjectsService.List).mockResolvedValue([
    project(),
    project({ ID: "p2", Name: "checkout", Kind: "linked", LifecycleState: "running" }),
  ]);
});

describe("ProjectSwitcher", () => {
  it("names the current project and its state", async () => {
    renderAt("/projects/p1");

    const trigger = await screen.findByRole("button", {
      name: "Project Etugora, switch project",
    });

    expect(trigger).toHaveTextContent("Etugora");
    expect(trigger).toHaveTextContent("Stopped");
  });

  it("leads back to every project", async () => {
    renderAt("/projects/p1");

    expect(await screen.findByRole("link", { name: "All projects" })).toHaveAttribute(
      "href",
      "/projects",
    );
  });

  it("lists every project with its state and marks the current one", async () => {
    const user = userEvent.setup();
    renderAt("/projects/p1");

    await user.click(await screen.findByRole("button", { name: /switch project/ }));

    const current = await screen.findByRole("link", { name: /Etugora/ });
    const other = screen.getByRole("link", { name: /checkout/ });

    expect(current).toHaveAttribute("aria-current", "true");
    expect(other).not.toHaveAttribute("aria-current");
    expect(other).toHaveTextContent("Running");
  });

  it("switches project and stays on the same section", async () => {
    const user = userEvent.setup();
    const router = renderAt("/projects/p1/services");

    await user.click(await screen.findByRole("button", { name: /switch project/ }));
    await user.click(await screen.findByRole("link", { name: /checkout/ }));

    expect(router.state.location.pathname).toBe("/projects/p2/services");
    expect(await screen.findByRole("button", { name: "Project checkout, switch project" })).toBeInTheDocument();
  });

  it("switches to the overview when no section is open", async () => {
    const user = userEvent.setup();
    const router = renderAt("/projects/p1");

    await user.click(await screen.findByRole("button", { name: /switch project/ }));
    await user.click(await screen.findByRole("link", { name: /checkout/ }));

    expect(router.state.location.pathname).toBe("/projects/p2");
  });

  it("says so when the project is no longer listed", async () => {
    renderAt("/projects/gone");

    expect(
      await screen.findByRole("button", { name: "Project unknown, switch project" }),
    ).toHaveTextContent("Unknown project");
  });
});
