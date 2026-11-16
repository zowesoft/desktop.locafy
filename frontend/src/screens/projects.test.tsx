import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as DialogsService from "../../bindings/locafy.dev/desktop/internal/dialogs/service";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import type { Project } from "../../bindings/locafy.dev/desktop/internal/projects/models";

import { ProjectsScreen } from "./projects";

vi.mock("../../bindings/locafy.dev/desktop/internal/dialogs/service", () => ({
  ChooseFolder: vi.fn(),
}));

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

  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  for (const fn of Object.values(ProjectsService)) {
    vi.mocked(fn).mockReset();
  }
  vi.mocked(DialogsService.ChooseFolder).mockReset();
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

  it("opens a project's workspace from its card", async () => {
    vi.mocked(ProjectsService.List).mockResolvedValue([project()]);

    renderWithQuery(<ProjectsScreen />);

    expect(await screen.findByRole("link", { name: "Etugora" })).toHaveAttribute(
      "href",
      "/projects/p1",
    );
    expect(screen.getByRole("link", { name: "Open Etugora" })).toHaveAttribute(
      "href",
      "/projects/p1",
    );
  });

  it("states each project's lifecycle in words", async () => {
    vi.mocked(ProjectsService.List).mockResolvedValue([
      project(),
      project({ ID: "p2", Name: "checkout", LifecycleState: "running" }),
    ]);

    renderWithQuery(<ProjectsScreen />);

    expect(await screen.findByText("Stopped")).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
  });

  it("filters projects by name or folder", async () => {
    const user = userEvent.setup();
    vi.mocked(ProjectsService.List).mockResolvedValue([
      project(),
      project({ ID: "p2", Name: "checkout", Kind: "linked", RootPath: "D:\\work\\shop" }),
    ]);

    renderWithQuery(<ProjectsScreen />);

    const filter = await screen.findByRole("searchbox", { name: "Filter projects" });
    await user.type(filter, "shop");

    expect(screen.getByText("checkout")).toBeInTheDocument();
    expect(screen.queryByText("Etugora")).toBeNull();
    expect(screen.getByText("1 of 2 projects")).toBeInTheDocument();

    await user.clear(filter);
    await user.type(filter, "nothing");

    expect(screen.getByText(/No project matches/)).toBeInTheDocument();
  });


  it("offers no filter before a project exists", async () => {
    renderWithQuery(<ProjectsScreen />);

    await screen.findByText("No projects yet");
    expect(screen.queryByRole("searchbox", { name: "Filter projects" })).toBeNull();
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

  it("fills the folder from the native picker", async () => {
    const user = userEvent.setup();
    vi.mocked(DialogsService.ChooseFolder).mockResolvedValue("D:\\work\\picked");
    vi.mocked(ProjectsService.Link).mockResolvedValue(
      project({ Kind: "linked", RootPath: "D:\\work\\picked" }),
    );

    renderWithQuery(<ProjectsScreen />);

    await user.click(screen.getByRole("button", { name: "Link folder" }));
    await user.click(await screen.findByRole("button", { name: /Browse/ }));

    expect(await screen.findByLabelText("Folder path")).toHaveValue("D:\\work\\picked");
    expect(DialogsService.ChooseFolder).toHaveBeenCalledWith("Link an existing folder");

    await user.click(screen.getByRole("button", { name: "Link folder" }));

    expect(ProjectsService.Link).toHaveBeenCalledWith("D:\\work\\picked");
  });

  it("keeps what was typed when the picker is cancelled", async () => {
    const user = userEvent.setup();
    vi.mocked(DialogsService.ChooseFolder).mockResolvedValue("");

    renderWithQuery(<ProjectsScreen />);

    await user.click(screen.getByRole("button", { name: "Link folder" }));
    await user.type(await screen.findByLabelText("Folder path"), "D:\\work\\typed");
    await user.click(screen.getByRole("button", { name: /Browse/ }));

    expect(DialogsService.ChooseFolder).toHaveBeenCalled();
    expect(screen.getByLabelText("Folder path")).toHaveValue("D:\\work\\typed");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("reports a picker that fails", async () => {
    const user = userEvent.setup();
    vi.mocked(DialogsService.ChooseFolder).mockRejectedValue(new Error("no window to attach to"));

    renderWithQuery(<ProjectsScreen />);

    await user.click(screen.getByRole("button", { name: "Link folder" }));
    await user.click(await screen.findByRole("button", { name: /Browse/ }));

    expect(await screen.findByRole("alert")).toHaveTextContent("no window to attach to");
  });

  it("offers the picker when changing a linked project's folder", async () => {
    const user = userEvent.setup();
    vi.mocked(ProjectsService.List).mockResolvedValue([
      project({ ID: "p2", Name: "checkout", Kind: "linked", RootPath: "D:\\work\\checkout" }),
    ]);
    vi.mocked(DialogsService.ChooseFolder).mockResolvedValue("D:\\work\\moved");

    renderWithQuery(<ProjectsScreen />);

    await user.click(await screen.findByRole("button", { name: "Change folder for checkout" }));
    await user.click(await screen.findByRole("button", { name: /Browse/ }));

    expect(await screen.findByLabelText("Folder path")).toHaveValue("D:\\work\\moved");
    expect(DialogsService.ChooseFolder).toHaveBeenCalledWith("Change project folder");
  });

  it("offers no picker when only naming a new project", async () => {
    const user = userEvent.setup();

    renderWithQuery(<ProjectsScreen />);

    await user.click(screen.getByRole("button", { name: "New project" }));
    await screen.findByLabelText("Project name");

    expect(screen.queryByRole("button", { name: /Browse/ })).toBeNull();
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
