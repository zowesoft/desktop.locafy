import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as ApplicationsService from "../../bindings/locafy.dev/desktop/internal/applications/service";
import * as NetworkService from "../../bindings/locafy.dev/desktop/internal/network/service";
import type { Domain } from "../../bindings/locafy.dev/desktop/internal/network/models";
import { ProjectDomains } from "./project-domains";

vi.mock("../../bindings/locafy.dev/desktop/internal/network/service", () => ({
  List: vi.fn(),
  ProxyStatus: vi.fn(),
  TrustPlan: vi.fn(),
  TrustCertificate: vi.fn(),
  Assign: vi.fn(),
  Remove: vi.fn(),
}));

vi.mock("../../bindings/locafy.dev/desktop/internal/applications/service", () => ({
  List: vi.fn(),
}));

const domain = (overrides: Partial<Domain> = {}): Domain => ({
  id: "d1",
  projectID: "p1",
  applicationID: "a1",
  applicationName: "Web server",
  directory: "apps/web",
  hostname: "api.etugora.test",
  port: 3000,
  createdAt: "2026-11-26T09:00:00Z",
  ...overrides,
});

function renderDomains() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <ProjectDomains projectID="p1" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.mocked(NetworkService.ProxyStatus).mockResolvedValue({
    running: true,
    httpAddress: "127.0.0.1:80",
    httpsAddress: "127.0.0.1:443",
    routes: 1,
    error: "",
  });
  vi.mocked(NetworkService.TrustPlan).mockResolvedValue({
    command: "certutil -user -addstore Root C:\\certs\\ca.crt",
    explanation: "This adds the certificate to your user's trusted root store.",
    certificate: "C:\\certs\\ca.crt",
  });
  vi.mocked(ApplicationsService.List).mockResolvedValue([]);
});

describe("ProjectDomains", () => {
  it("names the application and its folder beside the port", async () => {
    vi.mocked(NetworkService.List).mockResolvedValue([domain()]);
    renderDomains();

    const row = (await screen.findByText("api.etugora.test")).closest("tr");
    if (!row) throw new Error("the domain row was not rendered");

    expect(within(row).getByText("Web server")).toBeInTheDocument();
    expect(within(row).getByText("apps/web")).toBeInTheDocument();
    expect(within(row).getByText("3000")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Folder" })).toBeInTheDocument();
  });

  it("shows why no trust command can run instead of an unusable button", async () => {
    vi.mocked(NetworkService.List).mockResolvedValue([]);
    vi.mocked(NetworkService.TrustPlan).mockRejectedValue(
      new Error(
        "on Linux, trust the certificate yourself: add /home/me/.config/Locafy/certs/ca.crt to your distribution's trust store",
      ),
    );
    renderDomains();

    expect(
      await screen.findByText(/on Linux, trust the certificate yourself/),
    ).toBeInTheDocument();
  });
});
