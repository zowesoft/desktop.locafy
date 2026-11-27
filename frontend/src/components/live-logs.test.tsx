import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
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
  line: "vite ready in 240ms",
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
  it("lists the captured lines with their source and masks the credentials", async () => {
    vi.mocked(LogsService.Entries).mockResolvedValue([
      entry(),
      entry({ id: 2, layer: "service", sourceId: "svc-1", source: "PostgreSQL", line: "password=••• accepted", masked: true }),
    ]);

    renderLogs("p1");

    expect(await screen.findByText("vite ready in 240ms")).toBeInTheDocument();
    const lines = within(screen.getByRole("list"));
    expect(lines.getByText("PostgreSQL")).toBeInTheDocument();
    expect(lines.getByText("Etugora Web")).toBeInTheDocument();
    expect(lines.getByText("masked")).toBeInTheDocument();
    expect(screen.getByText(/1 with secrets masked/)).toBeInTheDocument();
    expect(LogsService.Entries).toHaveBeenCalledWith({
      projectId: "p1",
      layer: "",
      sourceId: "",
      contains: "",
      limit: 300,
    });
  });

  it("asks the engine for the chosen layer and the typed text", async () => {
    const user = userEvent.setup();
    renderLogs();

    await user.selectOptions(screen.getByLabelText("Filter by source kind"), "service");
    await user.type(screen.getByLabelText("Search log lines"), "ready");

    await waitFor(() =>
      expect(LogsService.Entries).toHaveBeenCalledWith({
        projectId: "",
        layer: "service",
        sourceId: "",
        contains: "ready",
        limit: 300,
      }),
    );
  });

  it("says nothing has been captured yet on an idle engine", async () => {
    renderLogs();

    expect(await screen.findByText(/No output captured yet/)).toBeInTheDocument();
  });
});
