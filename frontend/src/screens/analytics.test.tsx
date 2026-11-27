import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as AnalyticsService from "../../bindings/locafy.dev/desktop/internal/analytics/service";
import type { Trend } from "../../bindings/locafy.dev/desktop/internal/analytics/models";
import { AnalyticsScreen } from "./analytics";

vi.mock("../../bindings/locafy.dev/desktop/internal/analytics/service", () => ({
  Trend: vi.fn(),
  Retention: vi.fn(),
  SetRetention: vi.fn(),
}));

const trend = (overrides: Partial<Trend> = {}): Trend => ({
  days: [
    { day: "2026-11-24", count: 0 },
    { day: "2026-11-25", count: 3 },
    { day: "2026-11-26", count: 5 },
  ],
  actions: [
    { action: "Project started", count: 5 },
    { action: "Service added", count: 3 },
  ],
  total: 8,
  retainedDays: 90,
  ...overrides,
});

function renderAnalytics() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  render(
    <QueryClientProvider client={client}>
      <AnalyticsScreen />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.mocked(AnalyticsService.Trend).mockReset();
  vi.mocked(AnalyticsService.Trend).mockResolvedValue(trend());
  vi.mocked(AnalyticsService.Retention).mockReset();
  vi.mocked(AnalyticsService.Retention).mockResolvedValue(90);
  vi.mocked(AnalyticsService.SetRetention).mockReset();
  vi.mocked(AnalyticsService.SetRetention).mockResolvedValue();
});

describe("AnalyticsScreen", () => {
  it("shows the recorded totals and the action mix", async () => {
    renderAnalytics();

    expect(await screen.findByText("8")).toBeInTheDocument();
    expect(screen.getByText("in the last 30 days")).toBeInTheDocument();
    expect(await screen.findByText("Project started")).toBeInTheDocument();
    expect(screen.getByText("Service added")).toBeInTheDocument();
    expect(screen.getByText("90 days")).toBeInTheDocument();
    expect(AnalyticsService.Trend).toHaveBeenCalledWith(30);
  });

  it("says nothing has been recorded in an empty window", async () => {
    vi.mocked(AnalyticsService.Trend).mockResolvedValue(trend({ days: [{ day: "2026-11-26", count: 0 }], actions: [], total: 0 }));

    renderAnalytics();

    expect(await screen.findByText(/Nothing has been recorded in this window yet/)).toBeInTheDocument();
    expect(screen.getByText("No actions recorded in this window.")).toBeInTheDocument();
  });

  it("stores a new retention window", async () => {
    const user = userEvent.setup();
    renderAnalytics();

    await user.selectOptions(await screen.findByLabelText("Keep history for"), "30");

    expect(AnalyticsService.SetRetention).toHaveBeenCalledWith(30);
    await waitFor(() => expect(AnalyticsService.Trend).toHaveBeenCalled());
  });
});
