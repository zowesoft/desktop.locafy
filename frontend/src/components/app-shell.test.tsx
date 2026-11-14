import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

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
