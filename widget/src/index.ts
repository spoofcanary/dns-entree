import { mount, mountForVerify, destroy, isMounted } from "./mount";
import { checkDCReturn, restoreDCState, cleanReturnUrl } from "./screens/dc-flow";
import type { DnsEntreeOptions, RecordResult, DnsRecord } from "./types";

/**
 * Open the DNS Entree widget.
 * Creates a shadow DOM host, renders the modal, and starts provider detection.
 */
export function open(options: DnsEntreeOptions): void {
  if (!options.domain) {
    throw new Error("DnsEntree.open(): domain is required");
  }
  if (!options.records || options.records.length === 0) {
    throw new Error("DnsEntree.open(): at least one record is required");
  }
  if (!options.apiUrl) {
    throw new Error("DnsEntree.open(): apiUrl is required");
  }

  mount(options);
}

/** Close and destroy the widget. */
export function close(): void {
  destroy();
}

/** Returns true if the widget is currently open. */
export function isOpen(): boolean {
  return isMounted();
}

/**
 * Check for a Domain Connect return and auto-open the widget if found.
 * Call this on page load. If the page was returned to after a DC redirect,
 * the widget opens in verifying state and starts polling.
 *
 * Returns true if a DC return was detected and the widget was opened.
 */
export function handleDCReturn(options: Omit<DnsEntreeOptions, "domain" | "records">): boolean {
  const domain = checkDCReturn();
  if (!domain) return false;

  const saved = restoreDCState(domain);
  if (!saved) {
    cleanReturnUrl();
    return false;
  }

  // Clean the URL param
  cleanReturnUrl();

  // Build full options from saved state + caller overrides
  const fullOptions: DnsEntreeOptions = {
    ...options,
    domain: saved.domain,
    records: saved.records,
  };

  const results: RecordResult[] = saved.records.map((r) => ({
    record: r,
    status: "pending" as const,
  }));

  mountForVerify(fullOptions, results, saved.providerLabel);
  return true;
}

// Re-export types for ESM consumers
export type { DnsEntreeOptions, RecordResult, DnsRecord };
