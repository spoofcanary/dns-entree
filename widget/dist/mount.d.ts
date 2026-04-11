import type { DnsEntreeOptions, RecordResult } from "./types";
/**
 * Mount the widget. Creates a custom element host, attaches
 * a shadow root, and opens the modal inside it.
 */
export declare function mount(options: DnsEntreeOptions): void;
/**
 * Mount the widget directly into verify state (DC return flow).
 * Used when the page loads after a Domain Connect redirect.
 */
export declare function mountForVerify(options: DnsEntreeOptions, results: RecordResult[], providerLabel: string): void;
/** Destroy the widget. Removes the host element and shadow root immediately
 *  without waiting for the close animation (avoids race with double-mount). */
export declare function destroy(): void;
/** Returns true if the widget is currently mounted. */
export declare function isMounted(): boolean;
