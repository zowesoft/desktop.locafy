import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AppShell } from "./app-shell";

describe("AppShell top bar", () => {
  it("leaves status and notifications to the caller", () => {
    render(
      <AppShell>
        <p>content</p>
      </AppShell>,
    );

    // The shell owns its chrome, not the control set: a second, static status
    // pill or bell here would compete with the service-backed pair.
    expect(screen.queryByLabelText(/Notifications/)).toBeNull();
    expect(screen.queryByText(/Locafy running/i)).toBeNull();
    expect(screen.queryByText(/Status unavailable/i)).toBeNull();
  });

  it("renders the supplied controls once, next to the theme toggle", async () => {
    render(
      <AppShell topbarActions={<button type="button">Status slot</button>}>
        <p>content</p>
      </AppShell>,
    );

    expect(screen.getByText("Status slot")).toBeInTheDocument();
    expect(screen.getAllByText("Status slot")).toHaveLength(1);

    // Status reads before appearance: the supplied controls come first.
    const slot = screen.getByText("Status slot");
    const toggle = await screen.findByRole("button", { name: /Switch to (light|dark) theme/ });
    expect(slot.compareDocumentPosition(toggle) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});

describe("AppShell navigation", () => {
  const renderShell = () =>
    render(
      <AppShell>
        <p>content</p>
      </AppShell>,
    );

  it("lists only machine-level entries", () => {
    renderShell();

    const nav = within(screen.getByRole("navigation", { name: "Main" }));
    const labels = nav.getAllByRole("button").map((button) => button.textContent);

    expect(labels).toEqual([
      "Overview",
      "Projects",
      "Runtimes",
      "Downloads",
      "Analytics",
      "Logs",
      "Terminal",
      "Help & Support",
      "Settings",
    ]);
  });

  it("leaves project resources to the project workspace", () => {
    renderShell();

    const nav = within(screen.getByRole("navigation", { name: "Main" }));

    for (const name of ["Applications", "Services", "Domains"]) {
      expect(nav.queryByText(name)).toBeNull();
    }
  });

  it("shows planned entries as disabled instead of dropping them", () => {
    renderShell();

    const nav = screen.getByRole("navigation", { name: "Main" });
    const planned = nav.querySelectorAll('[aria-disabled="true"]');

    expect(Array.from(planned).map((item) => item.textContent)).toEqual([
      "TunnelsPlanned",
      "ExtensionsPlanned",
    ]);
  });
});

describe("AppShell search", () => {
  const renderShell = () => {
    const onNavigate = vi.fn();
    render(
      <AppShell onNavigate={onNavigate}>
        <p>content</p>
      </AppShell>,
    );

    return { onNavigate, field: screen.getByRole("combobox", { name: "Search" }) };
  };

  it("filters the navigation to the matching screens", async () => {
    const user = userEvent.setup();
    const { field } = renderShell();

    await user.type(field, "set");

    expect(await screen.findByRole("option", { name: "Settings" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Projects" })).toBeNull();
  });

  it("jumps to the highlighted screen on Enter", async () => {
    const user = userEvent.setup();
    const { onNavigate, field } = renderShell();

    await user.type(field, "set{Enter}");

    expect(onNavigate).toHaveBeenCalledWith("settings");
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("moves the highlight with the arrow keys", async () => {
    const user = userEvent.setup();
    const { onNavigate, field } = renderShell();

    await user.type(field, "lo");
    expect(await screen.findByRole("option", { name: "Downloads" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    await user.keyboard("{ArrowDown}{Enter}");

    expect(onNavigate).toHaveBeenCalledWith("logs");
  });

  it("does not offer a screen that is still planned", async () => {
    const user = userEvent.setup();
    const { field } = renderShell();

    await user.type(field, "tunn");

    expect(await screen.findByText(/No screens match/)).toBeInTheDocument();
    expect(screen.queryByRole("option")).toBeNull();
  });

  it("reaches the field with Ctrl K", async () => {
    renderShell();

    fireEvent.keyDown(window, { key: "k", ctrlKey: true });

    expect(screen.getByRole("combobox", { name: "Search" })).toHaveFocus();
  });

  it("clears the search with Escape", async () => {
    const user = userEvent.setup();
    const { field } = renderShell();

    await user.type(field, "set{Escape}");

    expect(field).toHaveValue("");
    expect(screen.queryByRole("listbox")).toBeNull();
  });
});

describe("AppShell theme toggle", () => {
  const renderShell = () =>
    render(
      <AppShell>
        <p>content</p>
      </AppShell>,
    );

  beforeEach(() => {
    document.documentElement.classList.add("dark");
    localStorage.clear();
  });

  afterEach(() => {
    document.documentElement.classList.remove("dark");
    localStorage.clear();
  });

  it("follows the theme already applied to the document", () => {
    renderShell();

    expect(screen.getByRole("button", { name: "Switch to light theme" })).toBeInTheDocument();
  });

  it("reads a light document as light", () => {
    document.documentElement.classList.remove("dark");
    renderShell();

    expect(screen.getByRole("button", { name: "Switch to dark theme" })).toBeInTheDocument();
  });

  it("switches to light and remembers the choice", async () => {
    const user = userEvent.setup();
    renderShell();

    await user.click(screen.getByRole("button", { name: "Switch to light theme" }));

    expect(document.documentElement.classList.contains("dark")).toBe(false);
    expect(localStorage.getItem("locafy-theme")).toBe("light");
    expect(screen.getByRole("button", { name: "Switch to dark theme" })).toBeInTheDocument();
  });

  it("switches back to dark", async () => {
    const user = userEvent.setup();
    document.documentElement.classList.remove("dark");
    renderShell();

    await user.click(screen.getByRole("button", { name: "Switch to dark theme" }));

    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(localStorage.getItem("locafy-theme")).toBe("dark");
    expect(screen.getByRole("button", { name: "Switch to light theme" })).toBeInTheDocument();
  });
});
