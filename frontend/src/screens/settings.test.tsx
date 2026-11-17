import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import * as SettingsService from "../../bindings/locafy.dev/desktop/internal/settings/service";
import * as StatusService from "../../bindings/locafy.dev/desktop/internal/status/service";
import { SettingsScreen } from "./settings";

vi.mock("../../bindings/locafy.dev/desktop/internal/status/service", () => ({
  NotificationPreferences: vi.fn(),
  SetNotificationPreferences: vi.fn(),
}));

vi.mock("../../bindings/locafy.dev/desktop/internal/settings/service", () => ({
  All: vi.fn(),
  Get: vi.fn(),
  SetString: vi.fn(),
  SetBool: vi.fn(),
  SetNumber: vi.fn(),
  SetJSON: vi.fn(),
  Delete: vi.fn(),
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

const defaultWorkspace = "C:\\locafy\\projects";

function renderWithQuery(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  });

  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

beforeEach(() => {
  vi.mocked(SettingsService.All).mockReset();
  vi.mocked(ProjectsService.WorkspaceDir).mockReset();
  vi.mocked(ProjectsService.SetWorkspaceDir).mockReset();
  vi.mocked(SettingsService.All).mockResolvedValue([]);
  vi.mocked(ProjectsService.WorkspaceDir).mockResolvedValue(defaultWorkspace);
  vi.mocked(ProjectsService.SetWorkspaceDir).mockResolvedValue(undefined);
  vi.mocked(StatusService.NotificationPreferences).mockReset();
  vi.mocked(StatusService.SetNotificationPreferences).mockReset();
  vi.mocked(StatusService.NotificationPreferences).mockResolvedValue({
    Machine: true,
    Project: true,
    Warnings: true,
    Sound: true,
  });
  vi.mocked(StatusService.SetNotificationPreferences).mockResolvedValue(undefined);
});

describe("SettingsScreen notifications", () => {
  it("lists each kind of notification with its current state", async () => {
    renderWithQuery(<SettingsScreen />);

    expect(await screen.findByRole("checkbox", { name: "Warnings" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Project activity" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Machine events" })).toBeChecked();
  });

  it("turns off one kind without disturbing the others", async () => {
    const user = userEvent.setup();
    renderWithQuery(<SettingsScreen />);

    await user.click(await screen.findByRole("checkbox", { name: "Project activity" }));

    expect(StatusService.SetNotificationPreferences).toHaveBeenCalledWith({
      Machine: true,
      Project: false,
      Warnings: true,
      Sound: true,
    });
  });

  it("shows the choice straight away instead of waiting for the round trip", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.SetNotificationPreferences).mockImplementation(
      () => new Promise(() => {}) as never,
    );

    renderWithQuery(<SettingsScreen />);

    await user.click(await screen.findByRole("checkbox", { name: "Machine events" }));

    expect(screen.getByRole("checkbox", { name: "Machine events" })).not.toBeChecked();
  });

  it("puts the choice back when it could not be saved", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.SetNotificationPreferences).mockRejectedValue(
      new Error("the database is read-only"),
    );

    renderWithQuery(<SettingsScreen />);

    const box = await screen.findByRole("checkbox", { name: "Project activity" });
    await user.click(box);

    expect(await screen.findByRole("alert")).toHaveTextContent("read-only");
    await waitFor(() => expect(box).toBeChecked());
  });

  it("shows a stored choice as it was saved", async () => {
    vi.mocked(StatusService.NotificationPreferences).mockResolvedValue({
      Machine: true,
      Project: false,
      Warnings: false,
      Sound: false,
    });

    renderWithQuery(<SettingsScreen />);

    expect(await screen.findByRole("checkbox", { name: "Project activity" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Warnings" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Machine events" })).toBeChecked();
  });

  it("reports a change that could not be saved", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.SetNotificationPreferences).mockRejectedValue(
      new Error("the database is read-only"),
    );

    renderWithQuery(<SettingsScreen />);

    await user.click(await screen.findByRole("checkbox", { name: "Machine events" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("read-only");
  });
});

describe("SettingsScreen workspace", () => {
  it("shows the folder managed projects are created in", async () => {
    renderWithQuery(<SettingsScreen />);

    expect(await screen.findByDisplayValue(defaultWorkspace)).toBeInTheDocument();
  });

  it("saves a new workspace folder", async () => {
    const user = userEvent.setup();
    renderWithQuery(<SettingsScreen />);

    const field = await screen.findByDisplayValue(defaultWorkspace);
    fireEvent.change(field, { target: { value: "D:\\locafy-projects" } });
    await user.click(screen.getByRole("button", { name: "Save workspace folder" }));

    expect(ProjectsService.SetWorkspaceDir).toHaveBeenCalledWith("D:\\locafy-projects");
  });

  it("restores the default workspace", async () => {
    const user = userEvent.setup();
    renderWithQuery(<SettingsScreen />);

    await screen.findByLabelText("Workspace folder");
    await user.click(screen.getByRole("button", { name: "Use default" }));

    expect(ProjectsService.SetWorkspaceDir).toHaveBeenCalledWith("");
  });

  it("reports a rejected folder", async () => {
    const user = userEvent.setup();
    vi.mocked(ProjectsService.SetWorkspaceDir).mockRejectedValue(
      new Error("the workspace folder must be an absolute path"),
    );

    renderWithQuery(<SettingsScreen />);

    const field = await screen.findByDisplayValue(defaultWorkspace);
    fireEvent.change(field, { target: { value: "projects" } });
    await user.click(screen.getByRole("button", { name: "Save workspace folder" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "must be an absolute path",
    );
  });

  it("does not offer to save an unchanged folder", async () => {
    renderWithQuery(<SettingsScreen />);

    await screen.findByLabelText("Workspace folder");

    expect(screen.getByRole("button", { name: "Save workspace folder" })).toBeDisabled();
  });
});

describe("SettingsScreen notification sound", () => {
  it("shows the chime as on by default", async () => {
    renderWithQuery(<SettingsScreen />);

    expect(await screen.findByRole("checkbox", { name: "Play a sound" })).toBeChecked();
  });

  it("turns the chime off without hiding any notification", async () => {
    const user = userEvent.setup();
    renderWithQuery(<SettingsScreen />);

    await user.click(await screen.findByRole("checkbox", { name: "Play a sound" }));

    expect(StatusService.SetNotificationPreferences).toHaveBeenCalledWith({
      Machine: true,
      Project: true,
      Warnings: true,
      Sound: false,
    });
  });
});
