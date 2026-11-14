import "@testing-library/jest-dom/vitest";
import { cleanup, configure } from "@testing-library/react";
import { afterEach } from "vitest";

afterEach(cleanup);

// Queries wait for state that usually lands in a microtask. The default second
// is tight when several jsdom environments run at once.
configure({ asyncUtilTimeout: 3000 });

// jsdom implements neither of these, and the popover primitives use them to
// measure and position their content.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

globalThis.ResizeObserver ??= ResizeObserverStub as unknown as typeof ResizeObserver;

window.matchMedia ??= ((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: () => {},
  removeListener: () => {},
  addEventListener: () => {},
  removeEventListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

Element.prototype.scrollIntoView ??= () => {};

// Pointer capture is part of user-event's click sequence; jsdom has no layout
// but the calls still need to exist.
Element.prototype.hasPointerCapture ??= () => false;
Element.prototype.setPointerCapture ??= () => {};
Element.prototype.releasePointerCapture ??= () => {};
