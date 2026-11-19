import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as RuntimesService from "../../bindings/locafy.dev/desktop/internal/runtimes/service";
import type { CatalogEntry } from "../../bindings/locafy.dev/desktop/internal/runtimes/models";
import { RuntimesScreen } from "./runtimes";

vi.mock("../../bindings/locafy.dev/desktop/internal/runtimes/service", () => ({
  Catalog: vi.fn(),
  KindFor: vi.fn(),
}));

const entry = (overrides: Partial<CatalogEntry> = {}): CatalogEntry => ({
  kind: { id: "php", label: "PHP", bin: "php" },
  copies: [],
  ...overrides,
});

function renderWithQuery(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

beforeEach(() => {
  vi.mocked(RuntimesService.Catalog).mockReset();
});

describe("RuntimesScreen", () => {
  it("lists every kind and each copy's version, path and source", async () => {
    vi.mocked(RuntimesService.Catalog).mockResolvedValue([
      entry({
        copies: [
          {
            id: "php@C:\\laragon\\bin\\php\\php.exe",
            kind: "php",
            version: "8.3.7",
            path: "C:\\laragon\\bin\\php\\php.exe",
            source: "installed",
          },
          {
            id: "php@C:\\tools\\php\\php.exe",
            kind: "php",
            version: "8.2.12",
            path: "C:\\tools\\php\\php.exe",
            source: "PATH",
          },
        ],
      }),
      entry({ kind: { id: "ruby", label: "Ruby", bin: "ruby" }, copies: [] }),
    ]);

    renderWithQuery(<RuntimesScreen />);

    expect(await screen.findByText("PHP")).toBeInTheDocument();
    expect(screen.getByText("8.3.7")).toBeInTheDocument();
    expect(screen.getByText(/laragon.bin.php.php.exe/i)).toBeInTheDocument();
    expect(screen.getByText("installed", { selector: "span" })).toBeInTheDocument();
    expect(screen.getByText("Ruby")).toBeInTheDocument();
    expect(screen.getByText(/No Ruby executable/)).toBeInTheDocument();
  });

  it("explains a failed read instead of pretending the machine is empty", async () => {
    vi.mocked(RuntimesService.Catalog).mockRejectedValue(new Error("machine gone"));

    renderWithQuery(<RuntimesScreen />);

    expect(await screen.findByRole("alert")).toHaveTextContent(/Could not read the runtimes/);
  });
});
