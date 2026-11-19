import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as DownloadsService from "../../bindings/locafy.dev/desktop/internal/downloads/service";
import type { Installable, Job } from "../../bindings/locafy.dev/desktop/internal/downloads/models";
import { DownloadsScreen } from "./downloads";

vi.mock("@wailsio/runtime", () => ({
  Events: {
    On: vi.fn(() => () => undefined),
  },
}));

vi.mock("../../bindings/locafy.dev/desktop/internal/downloads/service", () => ({
  List: vi.fn(),
  Installables: vi.fn(),
  Install: vi.fn(),
  Retry: vi.fn(),
  Cancel: vi.fn(),
  Remove: vi.fn(),
}));

const installable: Installable = {
  kind: "node",
  label: "Node.js 22.14.0",
  version: "22.14.0",
  spec: {
    kind: "node",
    version: "22.14.0",
    url: "https://nodejs.org/dist/v22.14.0/node-v22.14.0-win-x64.zip",
    format: "zip",
    mustContain: "node.exe",
    strip: 1,
  },
};

const job = (overrides: Partial<Job> = {}): Job => ({
  id: "j1",
  kindId: "node",
  label: "Node.js 22.14.0",
  state: "succeeded",
  progress: 0,
  total: 0,
  error: "",
  targetDir: "C:\\Users\\me\\AppData\\Roaming\\Locafy\\runtimes\\node\\22.14.0",
  createdAt: "2026-11-19T10:00:00Z",
  updatedAt: "2026-11-19T10:01:00Z",
  ...overrides,
});

function renderWithQuery(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

beforeEach(() => {
  vi.mocked(DownloadsService.List).mockReset();
  vi.mocked(DownloadsService.List).mockResolvedValue([]);
  vi.mocked(DownloadsService.Installables).mockReset();
  vi.mocked(DownloadsService.Installables).mockResolvedValue([installable]);
  vi.mocked(DownloadsService.Install).mockReset();
  vi.mocked(DownloadsService.Install).mockResolvedValue(job());
  vi.mocked(DownloadsService.Retry).mockReset();
  vi.mocked(DownloadsService.Retry).mockResolvedValue(job());
  vi.mocked(DownloadsService.Cancel).mockReset();
  vi.mocked(DownloadsService.Cancel).mockResolvedValue(undefined);
  vi.mocked(DownloadsService.Remove).mockReset();
  vi.mocked(DownloadsService.Remove).mockResolvedValue(undefined);
});

describe("DownloadsScreen", () => {
  it("lists the catalogue and every job with its outcome", async () => {
    vi.mocked(DownloadsService.List).mockResolvedValue([
      job({ state: "running", progress: 5 * 1024 * 1024, total: 30 * 1024 * 1024 }),
      job({ id: "j2", state: "failed", error: "the server answered 502 Bad Gateway" }),
    ]);

    renderWithQuery(<DownloadsScreen />);

    expect((await screen.findAllByText("Node.js 22.14.0")).length).toBeGreaterThan(0);
    await waitFor(() => {
      expect(screen.getByText("Installing…")).toBeInTheDocument();
    });
    expect(screen.getByText(/MB/)).toBeInTheDocument();
    expect(screen.getByText("the server answered 502 Bad Gateway")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Install 22.14.0" })).toBeInTheDocument();
  });

  it("starts an install for a version that is not installed", async () => {
    const user = userEvent.setup();
    vi.mocked(DownloadsService.List).mockResolvedValue([]);

    renderWithQuery(<DownloadsScreen />);

    await user.click(await screen.findByRole("button", { name: "Install 22.14.0" }));
    await waitFor(() => {
      expect(DownloadsService.Install).toHaveBeenCalledWith("node");
    });
  });

  it("retries a failed job and removes a finished one", async () => {
    const user = userEvent.setup();
    vi.mocked(DownloadsService.List).mockResolvedValue([
      job({ id: "j2", state: "failed", error: "the server answered 502 Bad Gateway" }),
      job({ id: "j3" }),
    ]);

    renderWithQuery(<DownloadsScreen />);

    await user.click(await screen.findByRole("button", { name: "Retry" }));
    await waitFor(() => {
      expect(DownloadsService.Retry).toHaveBeenCalledWith("j2");
    });

    const removeButtons = await screen.findAllByRole("button", { name: "Remove" });
    await user.click(removeButtons[0]);
    await waitFor(() => {
      expect(DownloadsService.Remove).toHaveBeenCalledWith("j2");
    });
  });

  it("shows an empty state when nothing is installed yet", async () => {
    vi.mocked(DownloadsService.Installables).mockResolvedValue([]);

    renderWithQuery(<DownloadsScreen />);

    expect(await screen.findByText(/The catalogue has nothing for this machine yet/)).toBeInTheDocument();
    expect(screen.getByText(/Nothing has been installed yet/)).toBeInTheDocument();
  });
});
