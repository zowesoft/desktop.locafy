import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vitest";

import { NotFound } from "./not-found";

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/" element={<p>overview</p>} />
        <Route path="*" element={<NotFound />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("NotFound", () => {
  it("names the route that was requested", () => {
    renderAt("/reports?range=week");

    expect(screen.getByRole("heading", { name: "Page not found" })).toBeInTheDocument();
    expect(screen.getByText("#/reports?range=week")).toBeInTheDocument();
  });

  it("returns to the overview", async () => {
    const user = userEvent.setup();
    renderAt("/reports");

    await user.click(screen.getByRole("button", { name: "Back to overview" }));

    expect(await screen.findByText("overview")).toBeInTheDocument();
  });
});
