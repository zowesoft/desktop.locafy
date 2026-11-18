import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as ScaffoldService from "../../bindings/locafy.dev/desktop/internal/scaffold/service";
import type { Job, Preflight, Recipe } from "../../bindings/locafy.dev/desktop/internal/scaffold/models";
import { ScaffoldForm } from "./scaffold-form";

const handlers = vi.hoisted(() => new Map<string, (event: { data: unknown }) => void>());

vi.mock("@wailsio/runtime", () => ({
  Events: {
    On: (name: string, callback: (event: { data: unknown }) => void) => {
      handlers.set(name, callback);
      return () => handlers.delete(name);
    },
  },
}));

vi.mock("../../bindings/locafy.dev/desktop/internal/scaffold/service", () => ({
  Recipes: vi.fn(),
  Preflight: vi.fn(),
  Start: vi.fn(),
  StartProject: vi.fn(),
  Cancel: vi.fn(),
  Discard: vi.fn(),
  Job: vi.fn(),
  InstallLaravelInstaller: vi.fn(),
}));

const recipe = (overrides: Partial<Recipe> = {}): Recipe => ({
  ID: "laravel",
  Label: "Laravel",
  Language: "PHP",
  Framework: "Laravel",
  Description: "PHP framework.",
  Docs: "https://laravel.com/docs",
  VerifiedAt: "Laravel 13.11.0, checked 2026-11-17",
  Options: [
    {
      Key: "version",
      Label: "Laravel version",
      Default: "13",
      Values: [
        { Value: "13", Label: "13 (latest)" },
        { Value: "12", Label: "12 (security fixes)" },
      ],
    },
  ],
  Requires: [],
  Commands: [],
  Verify: [],
  Application: { StartCommand: "php artisan serve", StopCommand: "" },
  ...overrides,
});

const preflight = (overrides: Partial<Preflight> = {}): Preflight => ({
  RecipeID: "laravel",
  Ready: true,
  Tools: [
    {
      Tool: "php",
      Label: "PHP",
      MinVersion: "8.3",
      Required: true,
      Candidates: [
        { Path: "C:\\php83\\php.exe", Version: "8.3.7", Source: "installed" },
        { Path: "C:\\php84\\php.exe", Version: "8.4.18", Source: "installed" },
      ],
      Selected: "C:\\php84\\php.exe",
      Version: "8.4.18",
      OK: true,
      Problem: "",
      Fix: null,
    },
  ],
  Commands: ["C:\\php84\\php.exe artisan serve"],
  Installer: true,
  Notes: [],
  ...overrides,
});

const job = (overrides: Partial<Job> = {}): Job => ({
  ID: "job-1",
  ProjectID: "p1",
  RecipeID: "laravel",
  Directory: "laravel",
  TargetPath: "C:\\locafy\\projects\\demo\\laravel",
  Status: "running",
  Error: "",
  LogPath: "",
  Commands: ["laravel new laravel"],
  CreatedTarget: true,
  CreatedAt: "",
  UpdatedAt: "",
  ...overrides,
});

beforeEach(() => {
  handlers.clear();
  vi.mocked(ScaffoldService.Recipes).mockReset();
  vi.mocked(ScaffoldService.Recipes).mockResolvedValue([
    recipe(),
    recipe({ ID: "nextjs", Label: "Next.js", Language: "TypeScript", Options: [] }),
    recipe({ ID: "vite", Label: "Vite", Language: "TypeScript", Options: [] }),
    recipe({ ID: "django", Label: "Django", Language: "Python", Options: [] }),
    recipe({ ID: "dotnet", Label: ".NET", Language: "C#", Options: [] }),
  ]);
  vi.mocked(ScaffoldService.Preflight).mockReset();
  vi.mocked(ScaffoldService.Preflight).mockResolvedValue(preflight());
  vi.mocked(ScaffoldService.Start).mockReset();
  vi.mocked(ScaffoldService.Start).mockResolvedValue(job());
  vi.mocked(ScaffoldService.StartProject).mockReset();
  vi.mocked(ScaffoldService.StartProject).mockResolvedValue(job());
  vi.mocked(ScaffoldService.Discard).mockReset();
  vi.mocked(ScaffoldService.Cancel).mockReset();
});

describe("ScaffoldForm", () => {
  it("lists the shipped frameworks and their versions", async () => {
    render(<ScaffoldForm mode="application" projectID="p1" onDone={() => {}} onCancel={() => {}} />);

    expect(await screen.findByRole("button", { name: /Laravel/ })).toHaveAttribute("aria-pressed", "true");
    for (const label of ["Next.js", "Vite", "Django", ".NET"]) {
      expect(screen.getByRole("button", { name: new RegExp(label) })).toBeInTheDocument();
    }
    const version = await screen.findByRole("combobox", { name: "Laravel version" });
    expect(version).toHaveValue("13");
  });

  it("shows the tools, the installed versions and the exact commands", async () => {
    render(<ScaffoldForm mode="application" projectID="p1" onDone={() => {}} onCancel={() => {}} />);

    expect(await screen.findByText(/8\.4\.18/)).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "PHP version" })).toHaveValue("C:\\php84\\php.exe");
    expect(screen.getByLabelText("Commands to run")).toHaveTextContent("artisan serve");
  });

  it("refuses to start while a required tool is missing", async () => {
    vi.mocked(ScaffoldService.Preflight).mockResolvedValue(
      preflight({
        Ready: false,
        Tools: [
          {
            Tool: "php",
            Label: "PHP",
            MinVersion: "8.3",
            Required: true,
            Candidates: [],
            Selected: "",
            Version: "",
            OK: false,
            Problem: "not installed",
            Fix: null,
          },
        ],
        Commands: [],
      }),
    );

    render(<ScaffoldForm mode="application" projectID="p1" onDone={() => {}} onCancel={() => {}} />);

    expect(await screen.findByText("not installed")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create application" })).toBeDisabled();
  });

  it("shows how to fix a tool that is missing", async () => {
    vi.mocked(ScaffoldService.Preflight).mockResolvedValue(
      preflight({
        Ready: false,
        Tools: [
          {
            Tool: "php",
            Label: "PHP",
            MinVersion: "8.3",
            Required: true,
            Candidates: [],
            Selected: "",
            Version: "",
            OK: false,
            Problem: "not installed",
            Fix: {
              Summary: "PHP 8.3 or newer is required and Locafy could not find it.",
              Command: "winget install --id PHP.PHP.8.4 -e",
              Link: "https://windows.php.net/download/",
              LinkLabel: "PHP downloads",
            },
          },
        ],
        Commands: [],
      }),
    );

    render(<ScaffoldForm mode="application" projectID="p1" onDone={() => {}} onCancel={() => {}} />);

    expect(await screen.findByText(/could not find it/)).toBeInTheDocument();
    expect(screen.getByText("winget install --id PHP.PHP.8.4 -e")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy the install command" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /PHP downloads/ })).toHaveAttribute(
      "href",
      "https://windows.php.net/download/",
    );
    expect(screen.getByRole("button", { name: "Create application" })).toBeDisabled();
  });

  it("checks the tools again after the operator installs one", async () => {
    const user = userEvent.setup();
    vi.mocked(ScaffoldService.Preflight)
      .mockResolvedValueOnce(
        preflight({
          Ready: false,
          Tools: [
            {
              Tool: "php",
              Label: "PHP",
              MinVersion: "8.3",
              Required: true,
              Candidates: [],
              Selected: "",
              Version: "",
              OK: false,
              Problem: "not installed",
              Fix: {
                Summary: "PHP 8.3 or newer is required and Locafy could not find it.",
                Command: "winget install --id PHP.PHP.8.4 -e",
                Link: "https://windows.php.net/download/",
                LinkLabel: "PHP downloads",
              },
            },
          ],
          Commands: [],
        }),
      )
      .mockResolvedValue(preflight());

    render(<ScaffoldForm mode="application" projectID="p1" onDone={() => {}} onCancel={() => {}} />);

    await screen.findByText("not installed");
    const before = vi.mocked(ScaffoldService.Preflight).mock.calls.length;

    await user.click(screen.getByRole("button", { name: /Check again/ }));

    await waitFor(() => expect(vi.mocked(ScaffoldService.Preflight).mock.calls.length).toBeGreaterThan(before));
    expect(await screen.findByText("Ready")).toBeInTheDocument();
  });

  it("starts with the chosen version and tool path, streams output and reports success", async () => {
    const onDone = vi.fn();
    render(<ScaffoldForm mode="application" projectID="p1" onDone={onDone} onCancel={() => {}} />);

    const user = userEvent.setup();
    const version = await screen.findByRole("combobox", { name: "Laravel version" });
    await user.selectOptions(version, "12");
    fireEvent.change(await screen.findByRole("combobox", { name: "PHP version" }), {
      target: { value: "C:\\php83\\php.exe" },
    });

    await waitFor(() => expect(ScaffoldService.Preflight).toHaveBeenCalledWith("laravel", { version: "12" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Create application" })).toBeEnabled());
    await user.click(screen.getByRole("button", { name: "Create application" }));

    await waitFor(() =>
      expect(ScaffoldService.Start).toHaveBeenCalledWith("p1", "laravel", "laravel", { version: "12" }, {
        php: "C:\\php83\\php.exe",
      }),
    );

    await act(async () => {
      handlers.get("scaffold:output")?.({ data: { jobId: "job-1", line: "Installing dependencies" } });
      handlers.get("scaffold:state")?.({ data: { jobId: "job-1", status: "succeeded", error: "" } });
    });

    expect(screen.getByLabelText("Installer output")).toHaveTextContent("Installing dependencies");
    expect(onDone).toHaveBeenCalled();
  });

  it("keeps a failure on screen and offers to remove the partial folder", async () => {
    vi.mocked(ScaffoldService.Discard).mockResolvedValue(undefined);
    const onDone = vi.fn();
    render(<ScaffoldForm mode="application" projectID="p1" onDone={onDone} onCancel={() => {}} />);

    const user = userEvent.setup();
    const start = await screen.findByRole("button", { name: "Create application" });
    await waitFor(() => expect(start).toBeEnabled());
    await user.click(start);
    await waitFor(() => expect(ScaffoldService.Start).toHaveBeenCalled());

    await act(async () => {
      handlers.get("scaffold:state")?.({
        data: { jobId: "job-1", status: "failed", error: "composer create-project failed" },
      });
    });

    expect(await screen.findByRole("alert")).toHaveTextContent("composer create-project failed");
    await user.click(screen.getByRole("button", { name: "Remove partial folder" }));
    expect(ScaffoldService.Discard).toHaveBeenCalledWith("job-1");
    expect(onDone).not.toHaveBeenCalled();
  });

  it("creates a project from a name in project mode", async () => {
    const user = userEvent.setup();
    render(<ScaffoldForm mode="project" onDone={() => {}} onCancel={() => {}} />);

    await user.type(await screen.findByRole("textbox", { name: "Project name" }), "Etugora");
    await user.click(screen.getByRole("button", { name: "Create project" }));

    await waitFor(() =>
      expect(ScaffoldService.StartProject).toHaveBeenCalledWith("Etugora", "laravel", { version: "13" }, {}),
    );
  });
});
