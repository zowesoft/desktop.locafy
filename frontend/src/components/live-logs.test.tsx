import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as LogsService from "../../bindings/locafy.dev/desktop/internal/logs/service";
import type { Entry } from "../../bindings/locafy.dev/desktop/internal/logs/models";
import { LiveLogs } from "./live-logs";

vi.mock("../../bindings/locafy.dev/desktop/internal/logs/service", () => ({
  Entries: vi.fn(),
}));

const entry = (overrides: Partial<Entry> = {}): Entry => ({
  id: 1,
  time: "2026-11-26T13:10:05Z",
  layer: "application",
  sourceId: "app-1",
  source: "Etugora Web",
  level: "info",
  line: "✓ Ready in 2.6s",
  masked: false,
  ...overrides,
});

function renderLogs(projectID?: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  render(
    <QueryClientProvider client={client}>
      <LiveLogs projectID={projectID} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.mocked(LogsService.Entries).mockReset();
  vi.mocked(LogsService.Entries).mockResolvedValue([]);
});

describe("LiveLogs", () => {
  it("shows each line with its time, source, severity and masked marker", async () => {
    vi.mocked(LogsService.Entries).mockResolvedValue([
      entry(),
      entry({
        id: 2,
        layer: "service",
        sourceId: "svc-1",
        source: "PostgreSQL",
        level: "warn",
        line: "password=••• accepted after retry",
        masked: true,
      }),
    ]);

    renderLogs("p1");

    expect(await screen.findByText("✓ Ready in 2.6s")).toBeInTheDocument();
    // The row reads as a date and a wall-clock time, not a raw instant.
    expect(screen.getAllByText(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/)).toHaveLength(2);
    expect(screen.getByText("info")).toBeInTheDocument();
    expect(screen.getByText("warn")).toBeInTheDocument();
    expect(screen.getAllByText("PostgreSQL").length).toBeGreaterThan(0);
    expect(screen.getByText("masked")).toBeInTheDocument();
    expect(screen.getByText(/1 masked/)).toBeInTheDocument();
    expect(LogsService.Entries).toHaveBeenCalledWith({
      projectId: "p1",
      layer: "",
      sourceId: "",
      level: "",
      contains: "",
      limit: 300,
    });
  });

  it("asks the engine for the chosen source kind, severity and text", async () => {
    const user = userEvent.setup();
    renderLogs();

    await user.selectOptions(screen.getByLabelText("Filter by source kind"), "service");
    await user.click(screen.getByRole("button", { name: "Error" }));
    await user.type(screen.getByLabelText("Search log lines"), "refused");

    await waitFor(() =>
      expect(LogsService.Entries).toHaveBeenCalledWith({
        projectId: "",
        layer: "service",
        sourceId: "",
        level: "error",
        contains: "refused",
        limit: 300,
      }),
    );
  });

  it("says nothing has been captured yet on an idle engine", async () => {
    renderLogs();

    expect(await screen.findByText("No output captured yet.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Export logs" })).toBeDisabled();
  });
});
