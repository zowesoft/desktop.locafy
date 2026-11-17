/*
 * The chime that announces new activity. It is synthesised rather than shipped
 * as an audio file: the bundle carries no player asset, nothing has to load
 * before it can be heard, and the volume is the operator's system volume.
 */
const notes = [
  { offset: 0, frequency: 784 }, // G5
  { offset: 0.12, frequency: 1047 }, // C6
] as const;

let context: AudioContext | null = null;

/**
 * Plays the two-note chime once. Audio is a courtesy, so every failure — no
 * audio device, a browser that blocks playback until a gesture, a suspended
 * context — is swallowed rather than surfacing as an error in the bell.
 */
export function playNotificationSound(): void {
  try {
    const Context =
      window.AudioContext ??
      (window as Window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    if (!Context) return;

    context ??= new Context();
    void context.resume();

    const start = context.currentTime;
    for (const note of notes) {
      const oscillator = context.createOscillator();
      const gain = context.createGain();
      oscillator.type = "sine";
      oscillator.frequency.value = note.frequency;
      // Ramped rather than switched on, so the note starts and ends without a
      // click; the peak is low because this is a background control plane.
      gain.gain.setValueAtTime(0.0001, start + note.offset);
      gain.gain.exponentialRampToValueAtTime(0.05, start + note.offset + 0.02);
      gain.gain.exponentialRampToValueAtTime(0.0001, start + note.offset + 0.18);
      oscillator.connect(gain).connect(context.destination);
      oscillator.start(start + note.offset);
      oscillator.stop(start + note.offset + 0.2);
    }
  } catch {
    // Silently ignore: a missing sound must never block the notification.
  }
}
