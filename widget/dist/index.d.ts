import type { DnsEntreeOptions, RecordResult, DnsRecord } from "./types";
/**
 * Open the DNS Entree widget.
 * Creates a shadow DOM host, renders the modal, and starts provider detection.
 */
export declare function open(options: DnsEntreeOptions): void;
/** Close and destroy the widget. */
export declare function close(): void;
/** Returns true if the widget is currently open. */
export declare function isOpen(): boolean;
/**
 * Check for a Domain Connect return and auto-open the widget if found.
 * Call this on page load. If the page was returned to after a DC redirect,
 * the widget opens in verifying state and starts polling.
 *
 * Returns true if a DC return was detected and the widget was opened.
 */
export declare function handleDCReturn(options: Omit<DnsEntreeOptions, "domain" | "records">): boolean;
export type { DnsEntreeOptions, RecordResult, DnsRecord };
