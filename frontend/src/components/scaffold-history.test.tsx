import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as DialogsService from "../../bindings/locafy.dev/desktop/internal/dialogs/service";
import * as ScaffoldService from "../../bindings/locafy.dev/desktop/internal/scaffold/service";
import type { JobSummary } from "../../bindings/locafy.dev/desktop/internal/scaffold/models";
import { ScaffoldHistoryDialog } from "./scaffold-history";

vi.mock("../../bindings/locafy.dev/desktop/internal/dialogs/service", () => ({
  OpenInTerminal: vi.fn(),
}));

vi.mock("../../bindings/locafy.dev/desktop/internal/scaffold/service", () => ({
  History: vi.fn(),
  JobLog: vi.fn(),
  OpenLog: vi.fn(),
}));

const job = (overrides: Partial<JobSummary> = {}): JobSummary => ({
  ID: "job-1",
  ProjectID: "",
  RecipeID: "laravel",
  RecipeLabel: "Laravel",
  Directory: ".",
  TargetPath: "C:\\locafy\\projects\\shop",
  Status: "failed",
  Error: "composer failed: could not resolve laravel/laravel",
  LogPath: "C:\\Users\\JohnCode\\AppData\\Roaming\\Locafy\\logs\\scaffold-job-1.log",
  LogAvailable: true,
  CreatedAt: "2026-11-18T09:00:00Z",
  UpdatedAt: "2026-11-18T09:01:00Z",
  ...overrides,
});

function renderDialog() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <ScaffoldHistoryDialog open onClose={() => {}} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.mocked(ScaffoldService.History).mockReset();
  vi.mocked(ScaffoldService.JobLog).mockReset();
  vi.mocked(ScaffoldService.OpenLog).mockReset();
  vi.mocked(ScaffoldService.History).mockResolvedValue([
    job(),
    job({
      ID: "job-2",
      RecipeID: "vite",
      RecipeLabel: "Vite",
      TargetPath: "C:\\locafy\\projects\\site",
      Status: "succeeded",
      Error: "",
    }),
  ]);
});

describe("ScaffoldHistoryDialog", () => {
  it("lists runs with their recipe, target, status and failure", async () => {
    renderDialog();

    expect(await screen.findByText("Laravel")).toBeInTheDocument();
    expect(screen.getByText("Vite")).toBeInTheDocument();
    expect(screen.getByText("C:\\locafy\\projects\\shop")).toBeInTheDocument();
    expect(screen.getByText("failed")).toBeInTheDocument();
    expect(screen.getByText("succeeded")).toBeInTheDocument();
    expect(screen.getByText(/could not resolve laravel\/laravel/)).toBeInTheDocument();
  });

  it("shows a saved run's output without opening a file", async () => {
    const user = userEvent.setup();
    vi.mocked(ScaffoldService.JobLog).mockResolvedValue("$ composer install\ncomposer failed");

    renderDialog();

    await user.click(
      await screen.findByRole("button", { name: "View the log of the Laravel run" }),
    );

    expect(ScaffoldService.JobLog).toHaveBeenCalledWith("job-1");
    expect(await screen.findByText(/\$ composer install/)).toBeInTheDocument();
  });

  it("opens the saved log file on request", async () => {
    const user = userEvent.setup();
    vi.mocked(ScaffoldService.JobLog).mockResolvedValue("output");
    vi.mocked(ScaffoldService.OpenLog).mockResolvedValue();

    renderDialog();

    await user.click(
      await screen.findByRole("button", { name: "Open the log file of the Laravel run" }),
    );

    expect(ScaffoldService.OpenLog).toHaveBeenCalledWith("job-1");
  });

  it("opens a terminal in the run's folder on request", async () => {
    const user = userEvent.setup();
    vi.mocked(DialogsService.OpenInTerminal).mockResolvedValue("Opened Windows Terminal");

    renderDialog();

    await user.click(
      await screen.findByRole("button", { name: "Open a terminal in the Laravel folder" }),
    );

    expect(DialogsService.OpenInTerminal).toHaveBeenCalledWith("C:\\locafy\\projects\\shop");
  });

  it("says when no install has run yet", async () => {
    vi.mocked(ScaffoldService.History).mockResolvedValue([]);

    renderDialog();

    expect(await screen.findByText(/No framework install has run yet/)).toBeInTheDocument();
  });

  it("reports a history that could not be read", async () => {
    vi.mocked(ScaffoldService.History).mockRejectedValue(new Error("database is locked"));

    renderDialog();

    expect(await screen.findByRole("alert")).toHaveTextContent("database is locked");
  });
});
