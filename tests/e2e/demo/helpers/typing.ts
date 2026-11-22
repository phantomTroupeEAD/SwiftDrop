import { Locator } from '@playwright/test';

/**
 * Types text into a Playwright locator with human-like keystroke pacing and subtle jitter.
 * Used for demonstrating natural human input (e.g. 1st dispatcher/courier).
 */
export async function slowType(
  locator: Locator,
  text: string,
  baseDelayMs: number = 65
): Promise<void> {
  await locator.focus();
  await locator.clear();
  for (const char of text) {
    const jitter = Math.floor(Math.random() * 30) - 15; // +/- 15ms variance
    const delay = Math.max(25, baseDelayMs + jitter);
    await locator.pressSequentially(char, { delay });
  }
}

/**
 * Types text rapidly into a Playwright locator.
 * Used for rapid entry of subsequent items (e.g. 2nd-5th couriers/dispatchers).
 */
export async function fastType(
  locator: Locator,
  text: string,
  delayMs: number = 10
): Promise<void> {
  await locator.focus();
  await locator.clear();
  await locator.pressSequentially(text, { delay: delayMs });
}
