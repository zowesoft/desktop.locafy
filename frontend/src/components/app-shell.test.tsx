import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

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

  it("renders the supplied controls once, next to the theme toggle", () => {
    render(
      <AppShell topbarActions={<button type="button">Status slot</button>}>
        <p>content</p>
      </AppShell>,
    );

    expect(screen.getByText("Status slot")).toBeInTheDocument();
    expect(screen.getAllByText("Status slot")).toHaveLength(1);
    expect(screen.getByRole("button", { name: /Switch to (light|dark) theme/ })).toBeInTheDocument();
  });
});
