import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import type { Project } from "../../bindings/locafy.dev/desktop/internal/projects/models";
import { ProjectsScreen } from "./projects";

vi.mock("../../bindings/locafy.dev/desktop/internal/projects/service", () => ({
  List: vi.fn(),
  Create: vi.fn(),
  Link: vi.fn(),
  Rename: vi.fn(),
  ChangeRoot: vi.fn(),
  Remove: vi.fn(),
  WorkspaceDir: vi.fn(),
  SetWorkspaceDir: vi.fn(),
}));

const project = (overrides: Partial<Project> = {}): Project => ({
  ID: "p1",
  Name: "Etugora",
  RootPath: "C:\\Users\\dev\\AppData\\Roaming\\Locafy\\projects\\etugora",
  Description: "",
  Kind: "managed",
  Status: "active",
  LifecycleState: "stopped",
  CreatedAt: "2026-11-14T09:00:00Z",
  UpdatedAt: "2026-11-14T09:00:00Z",
  ...overrides,
});

function renderWithQuery(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  });

  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

beforeEach(() => {
  for (const fn of Object.values(ProjectsService)) {
    vi.mocked(fn).mockReset();
  }
  vi.mocked(ProjectsService.WorkspaceDir).mockResolvedValue(
    "C:\\Users\\dev\\AppData\\Roaming\\Locafy\\projects",
  );
  vi.mocked(ProjectsService.List).mockResolvedValue([]);
});

describe("ProjectsScreen", () => {
  it("invites a first project and names the workspace", async () => {
    renderWithQuery(<ProjectsScreen />);

    expect(await screen.findByText("No projects yet")).toBeInTheDocument();
    expect(
      await screen.findByText("C:\\Users\\dev\\AppData\\Roaming\\Locafy\\projects"),
    ).toBeInTheDocument();
    expect(screen.getByText(/folder and files stay where they are/i)).toBeInTheDocument();
  });

  it("lists managed and linked projects with their folders", async () => {
    vi.mocked(ProjectsService.List).mockResolvedValue([
      project(),
      project({ ID: "p2", Name: "checkout", Kind: "linked", RootPath: "D:\\work\\checkout" }),
    ]);

    renderWithQuery(<ProjectsScreen />);

    expect(await screen.findByText("Etugora")).toBeInTheDocument();
    expect(screen.getByText("Managed")).toBeInTheDocument();
    expect(screen.getByText("Linked")).toBeInTheDocument();
    expect(screen.getByText("D:\\work\\checkout")).toBeInTheDocument();
    expect(screen.getByText("2 projects")).toBeInTheDocument();
  });

  it("creates a managed project from a name", async () => {
    const user = userEvent.setup();
    vi.mocked(ProjectsService.Create).mockResolvedValue(project());

    renderWithQuery(<ProjectsScreen />);

    await user.click(screen.getByRole("button", { name: "New project" }));
    await user.type(await screen.findByLabelText("Project name"), "Etugora");
    await user.click(screen.getByRole("button", { name: "Create project" }));

    expect(ProjectsService.Create).toHaveBeenCalledWith("Etugora");
  });

  it("links an existing folder by path", async () => {
    const user = userEvent.setup();
    vi.mocked(ProjectsService.Link).mockResolvedValue(
      project({ Kind: "linked", RootPath: "D:\\work\\checkout" }),
    );

    renderWithQuery(<ProjectsScreen />);

    await user.click(screen.getByRole("button", { name: "Link folder" }));
    await user.type(await screen.findByLabelText("Folder path"), "D:\\work\\checkout");
    await user.click(screen.getByRole("button", { name: "Link folder" }));

    expect(ProjectsService.Link).toHaveBeenCalledWith("D:\\work\\checkout");
  });

  it("explains that removing a project leaves its files alone", async () => {
    const user = userEvent.setup();
    vi.mocked(ProjectsService.List).mockResolvedValue([project()]);
    vi.mocked(ProjectsService.Remove).mockResolvedValue(undefined);

    renderWithQuery(<ProjectsScreen />);

    await user.click(await screen.findByRole("button", { name: "Remove Etugora" }));

    expect(await screen.findByText("Remove Etugora?")).toBeInTheDocument();
    expect(screen.getByText(/nothing is deleted/i)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Remove" }));

    expect(ProjectsService.Remove).toHaveBeenCalledWith("p1");
  });

  it("renames without touching the folder", async () => {
    const user = userEvent.setup();
    vi.mocked(ProjectsService.List).mockResolvedValue([project()]);
    vi.mocked(ProjectsService.Rename).mockResolvedValue(project({ Name: "Platform" }));

    renderWithQuery(<ProjectsScreen />);

    await user.click(await screen.findByRole("button", { name: "Rename Etugora" }));
    const field = await screen.findByLabelText("Project name");
    await user.clear(field);
    await user.type(field, "Platform");
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(ProjectsService.Rename).toHaveBeenCalledWith("p1", "Platform");
  });

  it("offers a folder change for linked projects only", async () => {
    vi.mocked(ProjectsService.List).mockResolvedValue([
      project(),
      project({ ID: "p2", Name: "checkout", Kind: "linked", RootPath: "D:\\work\\checkout" }),
    ]);

    renderWithQuery(<ProjectsScreen />);

    expect(await screen.findByRole("button", { name: "Change folder for checkout" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Change folder for Etugora" })).not.toBeInTheDocument();
  });

  it("reports a rejected action inside the dialog", async () => {
    const user = userEvent.setup();
    vi.mocked(ProjectsService.Create).mockRejectedValue(
      new Error("another project already uses this directory"),
    );

    renderWithQuery(<ProjectsScreen />);

    await user.click(screen.getByRole("button", { name: "New project" }));
    await user.type(await screen.findByLabelText("Project name"), "Etugora");
    await user.click(screen.getByRole("button", { name: "Create project" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "another project already uses this directory",
    );
  });
});
