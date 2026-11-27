import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as DiagnosticsService from "../../bindings/locafy.dev/desktop/internal/diagnostics/service";
import type { Bundle } from "../../bindings/locafy.dev/desktop/internal/diagnostics/models";
import { HelpScreen } from "./help";

const openURL = vi.fn();

vi.mock("@wailsio/runtime", () => ({
  Browser: { OpenURL: (...args: unknown[]) => openURL(...args) },
}));

vi.mock("../../bindings/locafy.dev/desktop/internal/diagnostics/service", () => ({
  Preview: vi.fn(),
  Export: vi.fn(),
}));

const bundle: Bundle = {
  generatedAt: "2026-11-27T10:00:00Z",
  files: [
    { name: "system.txt", content: "platform: windows/amd64\n", bytes: 23, truncated: false, masked: false },
    {
      name: "settings.txt",
      content: "database.url [string] = postgres://app:•••@localhost/db\n",
      bytes: 62,
      truncated: false,
      masked: true,
    },
  ],
};

beforeEach(() => {
  openURL.mockReset();
  vi.mocked(DiagnosticsService.Preview).mockReset();
  vi.mocked(DiagnosticsService.Preview).mockResolvedValue(bundle);
  vi.mocked(DiagnosticsService.Export).mockReset();
  vi.mocked(DiagnosticsService.Export).mockResolvedValue("C:\\Users\\me\\locafy-diagnostics.zip");
});

describe("HelpScreen", () => {
  it("searches the built-in articles without a network", async () => {
    const user = userEvent.setup();
    render(<HelpScreen />);

    expect(screen.getByText("Getting started")).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText("Search help articles…"), "port");

    expect(screen.getByText("Port already in use")).toBeInTheDocument();
    expect(screen.queryByText("Create your first project")).not.toBeInTheDocument();
  });

  it("opens the documentation in the system browser", async () => {
    const user = userEvent.setup();
    render(<HelpScreen />);

    await user.click(screen.getByRole("button", { name: /open docs/i }));

    expect(openURL).toHaveBeenCalledWith("https://locafy.dev/docs");
  });

  it("shows the masked bundle for review and saves only after confirmation", async () => {
    const user = userEvent.setup();
    render(<HelpScreen />);

    await user.click(screen.getByRole("button", { name: /review diagnostics/i }));

    expect(await screen.findByText("Review the diagnostics bundle")).toBeInTheDocument();
    expect(screen.getByText(/2 files/)).toBeInTheDocument();
    expect(screen.getByText(/platform: windows\/amd64/)).toBeInTheDocument();
    expect(DiagnosticsService.Export).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: /settings\.txt/ }));
    expect(screen.getByText(/postgres:\/\/app:•••@localhost\/db/)).toBeInTheDocument();
    expect(screen.getByText(/at least one value in this file was masked/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /save bundle/i }));

    await waitFor(() => expect(DiagnosticsService.Export).toHaveBeenCalledTimes(1));
    expect(await screen.findByText(/locafy-diagnostics\.zip/)).toBeInTheDocument();
  });

  it("keeps a cancelled save quiet and reports a real failure", async () => {
    const user = userEvent.setup();
    vi.mocked(DiagnosticsService.Export)
      .mockRejectedValueOnce("diagnostics export cancelled")
      .mockRejectedValueOnce("permission denied");
    render(<HelpScreen />);

    await user.click(screen.getByRole("button", { name: /review diagnostics/i }));
    await user.click(await screen.findByRole("button", { name: /save bundle/i }));
    await waitFor(() => expect(DiagnosticsService.Export).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /save bundle/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/permission denied/);
  });
});
