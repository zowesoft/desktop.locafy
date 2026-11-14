import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, Outlet, RouterProvider } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RouteError } from "./route-error";

// Throws on render, which is what a screen does when it cannot draw itself.
function Broken({ message }: { message: string }): never {
  throw new Error(message);
}

function Layout() {
  return (
    <>
      <p>overview</p>
      <Outlet />
    </>
  );
}

function renderRouter(routes: Parameters<typeof createMemoryRouter>[0], path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] });

  return render(<RouterProvider router={router} />);
}

let consoleError: ReturnType<typeof vi.spyOn>;
const swallowUncaught = (event: ErrorEvent) => event.preventDefault();

beforeEach(() => {
  window.addEventListener("error", swallowUncaught);
  // React reports the error it hands to the boundary through console.error and
  // reportError. The test asserts on what the screen shows, so both reports are
  // noise here; neither is what makes the boundary render.
  consoleError = vi.spyOn(console, "error").mockImplementation(() => {});
  vi.stubGlobal("reportError", () => {});
});

afterEach(() => {
  window.removeEventListener("error", swallowUncaught);
  consoleError.mockRestore();
  vi.unstubAllGlobals();
});

describe("RouteError", () => {
  it("names a thrown failure and shows its message", async () => {
    renderRouter(
      [
        {
          path: "/",
          element: <Layout />,
          errorElement: <RouteError />,
          children: [
            { path: "broken", element: <Broken message="Canvas failed to initialise" /> },
          ],
        },
      ],
      "/broken",
    );

    expect(
      await screen.findByRole("heading", { name: "This screen could not be displayed" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Canvas failed to initialise")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Reload/ })).toBeInTheDocument();
    expect(screen.getByText("Technical details")).toBeInTheDocument();
  });

  it("reports the status of a failed route response", async () => {
    renderRouter(
      [
        {
          path: "/",
          element: <Layout />,
          errorElement: <RouteError />,
          children: [
            {
              path: "missing",
              element: <p>never rendered</p>,
              loader: () => {
                throw new Response(null, { status: 404, statusText: "Not Found" });
              },
            },
          ],
        },
      ],
      "/missing",
    );

    expect(await screen.findByRole("heading", { name: "404 Not Found" })).toBeInTheDocument();
    expect(screen.getByText("The route did not return a usable response.")).toBeInTheDocument();
    expect(screen.getByText("Response status 404.")).toBeInTheDocument();
  });

  it("returns to the overview", async () => {
    const user = userEvent.setup();
    renderRouter(
      [
        {
          path: "/",
          element: <Layout />,
          errorElement: <RouteError />,
          children: [{ path: "broken", element: <Broken message="Something broke" /> }],
        },
      ],
      "/broken",
    );

    await user.click(await screen.findByRole("button", { name: "Back to overview" }));

    expect(await screen.findByText("overview")).toBeInTheDocument();
  });
});
