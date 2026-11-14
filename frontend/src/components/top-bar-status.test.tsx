import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as StatusService from "../../bindings/locafy.dev/desktop/internal/status/service";
import type { Health, Notification } from "../../bindings/locafy.dev/desktop/internal/status/models";
import { EngineStatus, Notifications } from "./top-bar-status";

vi.mock("../../bindings/locafy.dev/desktop/internal/status/service", () => ({
  Health: vi.fn(),
  Notifications: vi.fn(),
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

const notification = (id: string, action: string, minutesAgo: number): Notification => ({
  ID: id,
  Scope: "machine",
  Action: action,
  Detail: "",
  CreatedAt: new Date(Date.now() - minutesAgo * 60_000).toISOString(),
});

function renderWithQuery(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });

  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

beforeEach(() => {
  vi.mocked(StatusService.Health).mockReset();
  vi.mocked(StatusService.Notifications).mockReset();
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
