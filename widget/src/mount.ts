import { Modal } from "./modal";
import type { DnsEntreeOptions, RecordResult } from "./types";

const HOST_TAG = "dns-entree-widget";

let currentModal: Modal | null = null;
let hostElement: HTMLElement | null = null;

/**
 * Mount the widget. Creates a custom element host, attaches
 * a shadow root, and opens the modal inside it.
 */
export function mount(options: DnsEntreeOptions): void {
  // Prevent double-mount
  if (currentModal) {
    destroy();
  }

  // Create host element
  hostElement = document.createElement(HOST_TAG);
  document.body.appendChild(hostElement);

  // Attach shadow root
  const shadow = hostElement.attachShadow({ mode: "open" });

  // Create and open modal
  currentModal = new Modal(shadow, options);
  currentModal.open();
}

/**
 * Mount the widget directly into verify state (DC return flow).
 * Used when the page loads after a Domain Connect redirect.
 */
export function mountForVerify(
  options: DnsEntreeOptions,
  results: RecordResult[],
  providerLabel: string,
): void {
  // Prevent double-mount
  if (currentModal) {
    destroy();
  }

  // Create host element
  hostElement = document.createElement(HOST_TAG);
  document.body.appendChild(hostElement);

  // Attach shadow root
  const shadow = hostElement.attachShadow({ mode: "open" });

  // Create modal and open directly into verify state
  currentModal = new Modal(shadow, options);
  currentModal.openForVerify(results, providerLabel);
}

/** Destroy the widget. Removes the host element and shadow root immediately
 *  without waiting for the close animation (avoids race with double-mount). */
export function destroy(): void {
  const modal = currentModal;
  const host = hostElement;
  currentModal = null;
  hostElement = null;

  if (host) {
    host.remove();
  }
  // Modal reference is now detached - no close() call needed since the
  // shadow root was removed with the host element. This avoids the race
  // where close()'s transitionend fires after the new modal is mounted.
  void modal;
}

/** Returns true if the widget is currently mounted. */
export function isMounted(): boolean {
  return currentModal !== null;
}
