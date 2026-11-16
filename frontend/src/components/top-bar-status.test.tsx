import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import * as StatusService from "../../bindings/locafy.dev/desktop/internal/status/service";
import type { Health, Notification } from "../../bindings/locafy.dev/desktop/internal/status/models";
import { EngineStatus, Notifications } from "./top-bar-status";

vi.mock("../../bindings/locafy.dev/desktop/internal/status/service", () => ({
  Health: vi.fn(),
  Notifications: vi.fn(),
  Dismiss: vi.fn(),
  ClearNotifications: vi.fn(),
  OpenDataFolder: vi.fn(),
  NotificationPreferences: vi.fn(),
}));

const health: Health = {
  Name: "Locafy",
  Healthy: true,
  Summary: "All systems operational",
  Hostname: "studio",
  Platform: "windows",
  Architecture: "amd64",
  DatabasePath: "C:\\Users\\dev\\AppData\\Roaming\\Locafy\\data\\locafy.db",
  DatabaseOK: true,
  StartedAt: "2026-11-14T09:00:00Z",
  UptimeSeconds: 7500,
  CPUPercent: 42,
  Memory: { UsedBytes: 2 * 1024 ** 3, TotalBytes: 8 * 1024 ** 3, Percent: 25 },
  Disk: { UsedBytes: 280 * 1024 ** 3, TotalBytes: 500 * 1024 ** 3, Percent: 56 },
};

const notification = (
  id: string,
  action: string,
  minutesAgo: number,
  severity = "info",
): Notification => ({
  ID: id,
  Scope: "machine",
  ScopeID: "",
  Action: action,
  Detail: "",
  CreatedAt: new Date(Date.now() - minutesAgo * 60_000).toISOString(),
  Severity: severity,
  Dismissible: severity === "info",
  OpenPath: "",
});

function renderWithQuery(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });

  return render(
    <MemoryRouter>
      <QueryClientProvider client={client}>{ui}</QueryClientProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  sessionStorage.clear();
  vi.mocked(StatusService.Health).mockReset();
  vi.mocked(StatusService.Notifications).mockReset();
  vi.mocked(StatusService.Dismiss).mockReset();
  vi.mocked(StatusService.ClearNotifications).mockReset();
  vi.mocked(StatusService.OpenDataFolder).mockReset();
  vi.mocked(StatusService.NotificationPreferences).mockReset();
  vi.mocked(StatusService.NotificationPreferences).mockResolvedValue({
    Machine: true,
    Project: true,
    Warnings: true,
  });
});

describe("EngineStatus", () => {
  it("reports a healthy engine with its CPU usage", async () => {
    vi.mocked(StatusService.Health).mockResolvedValue(health);

    renderWithQuery(<EngineStatus />);

    const trigger = await screen.findByRole("button", { name: /Locafy running/ });
    expect(trigger).toHaveTextContent("42% CPU");
  });

  it("shows a checking state before the first answer arrives", () => {
    // The binding returns a cancellable promise; a plain pending promise is
    // all the component needs to render its loading state.
    vi.mocked(StatusService.Health).mockReturnValue(
      new Promise(() => {}) as unknown as ReturnType<typeof StatusService.Health>,
    );

    renderWithQuery(<EngineStatus />);

    expect(screen.getByRole("button", { name: /Checking/ })).toBeInTheDocument();
  });

  it("reports an unhealthy engine as needing attention", async () => {
    vi.mocked(StatusService.Health).mockResolvedValue({
      ...health,
      Healthy: false,
      DatabaseOK: false,
      Summary: "Database unavailable",
    });

    renderWithQuery(<EngineStatus />);

    const trigger = await screen.findByRole("button", { name: /Needs attention/ });
    expect(trigger).not.toHaveTextContent("% CPU");
  });

  it("reports an unreachable backend instead of claiming health", async () => {
    vi.mocked(StatusService.Health).mockRejectedValue(new Error("runtime unavailable"));

    renderWithQuery(<EngineStatus />);

    expect(await screen.findByRole("button", { name: /Status unavailable/ })).toBeInTheDocument();
  });

  it("explains an unreachable backend in its details", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Health).mockRejectedValue(new Error("runtime unavailable"));

    renderWithQuery(<EngineStatus />);

    await user.click(await screen.findByRole("button", { name: /Status unavailable/ }));

    expect(
      await screen.findByText("The engine did not respond to a status request."),
    ).toBeInTheDocument();
  });

  it("lists machine and resource detail when opened", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Health).mockResolvedValue(health);

    renderWithQuery(<EngineStatus />);

    await user.click(await screen.findByRole("button", { name: /Locafy running/ }));

    expect(await screen.findByText("studio")).toBeInTheDocument();
    expect(screen.getByText("windows · amd64")).toBeInTheDocument();
    expect(screen.getByText("2h 5m")).toBeInTheDocument();
    expect(screen.getByText("2.0 GB of 8.0 GB in use")).toBeInTheDocument();
    expect(screen.getByText("280 GB of 500 GB on the data volume")).toBeInTheDocument();
    expect(screen.getByRole("progressbar", { name: "Memory" })).toHaveAttribute(
      "aria-valuenow",
      "25",
    );
    expect(screen.getByRole("progressbar", { name: "Disk" })).toHaveAttribute(
      "aria-valuenow",
      "56",
    );
  });
});

describe("Notifications", () => {
  it("says so when nothing has been recorded", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockResolvedValue([]);

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications" }));

    expect(await screen.findByText("Nothing recorded yet.")).toBeInTheDocument();
  });

  it("blames the settings rather than the history when every type is off", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockResolvedValue([]);
    vi.mocked(StatusService.NotificationPreferences).mockResolvedValue({
      Machine: false,
      Project: false,
      Warnings: false,
    });

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications" }));

    expect(
      await screen.findByText(/Every notification type is turned off/),
    ).toBeInTheDocument();
    expect(screen.queryByText("Nothing recorded yet.")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Settings" })).toHaveAttribute(
      "href",
      "/settings",
    );
  });

  it("still reports an empty history while some types are on", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockResolvedValue([]);
    vi.mocked(StatusService.NotificationPreferences).mockResolvedValue({
      Machine: true,
      Project: false,
      Warnings: false,
    });

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications" }));

    expect(await screen.findByText("Nothing recorded yet.")).toBeInTheDocument();
  });

  it("shows recent entries with their action and age", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      notification("a", "Machine registered", 5),
      notification("b", "Engine started", 120),
    ]);

    renderWithQuery(<Notifications />);

    const trigger = await screen.findByRole("button", {
      name: "Notifications, 2 recent entries",
    });
    await user.click(trigger);

    expect(await screen.findByText("Machine registered")).toBeInTheDocument();
    expect(screen.getByText("Engine started")).toBeInTheDocument();
    expect(screen.getByText("5m ago")).toBeInTheDocument();
    expect(screen.getByText("2h ago")).toBeInTheDocument();
  });

  it("marks a warning and counts it in the bell's name", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      notification("disk-space", "Low disk space", 0, "warning"),
      notification("b", "Engine started", 30),
    ]);

    renderWithQuery(<Notifications />);

    const trigger = await screen.findByRole("button", {
      name: "Notifications, 2 recent entries, 1 warning",
    });
    await user.click(trigger);

    expect(await screen.findByText("Low disk space")).toBeInTheDocument();
    expect(screen.getByText("Warning")).toBeInTheDocument();
    expect(screen.getByText("Engine started")).toBeInTheDocument();
  });

  it("leaves the bell name unchanged when nothing needs acting on", async () => {
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      notification("a", "Machine registered", 5),
    ]);

    renderWithQuery(<Notifications />);

    expect(
      await screen.findByRole("button", { name: "Notifications, 1 recent entries" }),
    ).toBeInTheDocument();
  });

  it("caps the badge at nine plus", async () => {
    vi.mocked(StatusService.Notifications).mockResolvedValue(
      Array.from({ length: 12 }, (_, index) => notification(String(index), `Action ${index}`, 1)),
    );

    renderWithQuery(<Notifications />);

    const trigger = await screen.findByRole("button", {
      name: "Notifications, 12 recent entries",
    });
    expect(trigger).toHaveTextContent("9+");
  });

  it("shows project actions with the project they belong to", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      { ...notification("p1", "Project created", 2), Scope: "project", Detail: "Etugora" },
      { ...notification("p2", "Project removed", 9), Scope: "project", Detail: "checkout" },
    ]);

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications, 2 recent entries" }));

    expect(await screen.findByText("Project created")).toBeInTheDocument();
    expect(screen.getByText("project · Etugora")).toBeInTheDocument();
    expect(screen.getByText("Project removed")).toBeInTheDocument();
    expect(screen.getByText("project · checkout")).toBeInTheDocument();
  });

  it("keeps entry ages current while the bell is open and names the exact time", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(new Date("2026-11-16T10:00:00Z"));
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      notification("a", "Machine registered", 0),
    ]);

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications, 1 recent entries" }));

    const age = screen.getByText("just now");
    expect(age).toHaveAttribute("title", new Date("2026-11-16T10:00:00Z").toLocaleString());

    act(() => {
      vi.advanceTimersByTime(120_000);
    });

    await waitFor(() => expect(screen.getByText("2m ago")).toBeInTheDocument());
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("links a project entry to that project and leaves other entries alone", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      { ...notification("p1", "Project created", 2), Scope: "project", ScopeID: "p1", Detail: "Etugora" },
      notification("m1", "Engine started", 20),
    ]);

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications, 2 recent entries" }));

    expect(await screen.findByRole("link", { name: "Open project Etugora" })).toHaveAttribute(
      "href",
      "/projects/p1",
    );
    expect(screen.getByText("Engine started")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Open project/ })).not.toBeNull();
  });

  it("asks for a larger page when the list is full", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockResolvedValue(
      Array.from({ length: 10 }, (_, index) => notification(String(index), `Action ${index}`, index + 1)),
    );

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications, 10 recent entries" }));
    await user.click(await screen.findByRole("button", { name: "Show more activity" }));

    expect(StatusService.Notifications).toHaveBeenLastCalledWith(30);
  });

  it("keeps the grown page when the bell is reopened from scratch", async () => {
    const user = userEvent.setup();
    const history = Array.from({ length: 30 }, (_, index) =>
      notification(String(index), `Action ${index}`, index + 1),
    );
    // The generated binding is a cancellable promise; the bell only awaits it.
    vi.mocked(StatusService.Notifications).mockImplementation(
      (async (limit: number) => history.slice(0, limit)) as never,
    );

    const first = renderWithQuery(<Notifications />);
    await user.click(await screen.findByRole("button", { name: "Notifications, 10 recent entries" }));
    await user.click(await screen.findByRole("button", { name: "Show more activity" }));
    await waitFor(() => expect(StatusService.Notifications).toHaveBeenLastCalledWith(30));
    first.unmount();

    vi.mocked(StatusService.Notifications).mockClear();
    renderWithQuery(<Notifications />);

    await waitFor(() => expect(StatusService.Notifications).toHaveBeenLastCalledWith(30));
    expect(
      await screen.findByRole("button", { name: "Notifications, 30 recent entries" }),
    ).toBeInTheDocument();
  });

  it("starts at ten again when the saved page is unusable", async () => {
    const user = userEvent.setup();
    sessionStorage.setItem("locafy.notifications.page", "nonsense");
    vi.mocked(StatusService.Notifications).mockResolvedValue([notification("a", "Machine registered", 5)]);

    renderWithQuery(<Notifications />);

    await waitFor(() => expect(StatusService.Notifications).toHaveBeenLastCalledWith(10));
    await user.click(await screen.findByRole("button", { name: "Notifications, 1 recent entries" }));
    expect(screen.queryByRole("button", { name: "Show more activity" })).not.toBeInTheDocument();
  });

  it("offers no larger page when everything fits", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      notification("a", "Machine registered", 5),
      notification("b", "Engine started", 120),
    ]);

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications, 2 recent entries" }));

    expect(screen.queryByRole("button", { name: "Show more activity" })).not.toBeInTheDocument();
  });

  it("dismisses a single entry", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Dismiss).mockResolvedValue(undefined);
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      notification("a", "Machine registered", 5),
      notification("b", "Engine started", 30),
    ]);

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications, 2 recent entries" }));
    await user.click(await screen.findByRole("button", { name: "Dismiss Engine started" }));

    expect(StatusService.Dismiss).toHaveBeenCalledWith("b");
  });

  it("clears every entry at once", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.ClearNotifications).mockResolvedValue(2);
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      notification("a", "Machine registered", 5),
      notification("b", "Engine started", 30),
    ]);

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications, 2 recent entries" }));
    await user.click(await screen.findByRole("button", { name: "Clear notifications" }));

    expect(StatusService.ClearNotifications).toHaveBeenCalled();
  });

  it("reports a failed dismissal instead of pretending it worked", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      notification("a", "Machine registered", 5),
    ]);
    vi.mocked(StatusService.Dismiss).mockRejectedValue(new Error("notification not found"));

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications, 1 recent entries" }));
    await user.click(await screen.findByRole("button", { name: "Dismiss Machine registered" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("notification not found");
  });

  it("does not offer to dismiss a warning derived from the machine", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      notification("disk-space", "Low disk space", 0, "warning"),
      notification("b", "Engine started", 30),
    ]);

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: /Notifications, 2 recent entries/ }));

    expect(
      screen.queryByRole("button", { name: "Dismiss Low disk space" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Dismiss Engine started" })).toBeInTheDocument();
  });

  it("opens the folder the disk warning is about", async () => {
    const user = userEvent.setup();
    const dataFolder = "C:\\Users\\dev\\AppData\\Roaming\\Locafy\\data";
    vi.mocked(StatusService.OpenDataFolder).mockResolvedValue(undefined);
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      { ...notification("disk-space", "Low disk space", 0, "warning"), OpenPath: dataFolder },
    ]);

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: /Notifications, 1 recent entries/ }));

    const open = await screen.findByRole("button", { name: "Open data folder" });
    expect(open).toHaveAttribute("title", dataFolder);

    await user.click(open);

    expect(StatusService.OpenDataFolder).toHaveBeenCalled();
  });

  it("reports a folder that could not be opened", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.OpenDataFolder).mockRejectedValue(new Error("explorer is unavailable"));
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      {
        ...notification("disk-space", "Low disk space", 0, "warning"),
        OpenPath: "C:\\Users\\dev\\AppData\\Roaming\\Locafy\\data",
      },
    ]);

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: /Notifications, 1 recent entries/ }));
    await user.click(await screen.findByRole("button", { name: "Open data folder" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("explorer is unavailable");
  });

  it("offers no folder for entries with nothing to open", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockResolvedValue([
      notification("a", "Machine registered", 5),
    ]);

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications, 1 recent entries" }));

    expect(screen.queryByRole("button", { name: "Open data folder" })).not.toBeInTheDocument();
  });

  it("surfaces a failure to read activity", async () => {
    const user = userEvent.setup();
    vi.mocked(StatusService.Notifications).mockRejectedValue(new Error("database is locked"));

    renderWithQuery(<Notifications />);

    await user.click(await screen.findByRole("button", { name: "Notifications" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Could not read activity: Error: database is locked",
    );
  });
});
