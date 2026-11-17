import { describe, expect, it, vi } from "vitest";

function fakeAudioContext() {
  const notes: number[] = [];

  class FakeContext {
    currentTime = 1;
    destination = {};
    resume = vi.fn().mockResolvedValue(undefined);

    createOscillator() {
      const frequency = { value: 0 };
      return {
        type: "sine",
        frequency,
        connect: (node: unknown) => node,
        start: () => notes.push(frequency.value),
        stop: () => {},
      };
    }

    createGain() {
      return {
        gain: {
          setValueAtTime: () => {},
          exponentialRampToValueAtTime: () => {},
        },
        connect: (node: unknown) => node,
      };
    }
  }

  return { FakeContext, notes };
}

describe("playNotificationSound", () => {
  it("plays a rising two-note chime", async () => {
    const { FakeContext, notes } = fakeAudioContext();
    vi.stubGlobal("AudioContext", FakeContext);
    vi.resetModules();

    try {
      const sound = await import("./notification-sound");
      sound.playNotificationSound();
    } finally {
      vi.unstubAllGlobals();
    }

    expect(notes).toEqual([784, 1047]);
  });

  it("stays silent where audio is unavailable", async () => {
    vi.stubGlobal("AudioContext", undefined);
    vi.resetModules();

    try {
      const sound = await import("./notification-sound");
      expect(() => sound.playNotificationSound()).not.toThrow();
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("swallows a refused audio context", async () => {
    class RefusingContext {
      constructor() {
        throw new Error("no audio device");
      }
    }
    vi.stubGlobal("AudioContext", RefusingContext);
    vi.resetModules();

    try {
      const sound = await import("./notification-sound");
      expect(() => sound.playNotificationSound()).not.toThrow();
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
