import type { DCDiscoverData } from "../api";
import type { DnsEntreeOptions, DnsRecord } from "../types";
export interface DCFlowState {
    domain: string;
    records: DnsRecord[];
    dcDiscovery: DCDiscoverData;
    providerLabel: string;
    timestamp: number;
}
/** Save widget state to sessionStorage for resume after DC redirect. */
export declare function saveDCState(options: DnsEntreeOptions, dcDiscovery: DCDiscoverData, providerLabel: string): void;
/** Restore saved state from sessionStorage. */
export declare function restoreDCState(domain: string): DCFlowState | null;
/** Clean up saved state. */
export declare function clearDCState(domain: string): void;
/**
 * Build the DC flow screen DOM.
 * Shows provider name and "Set up automatically" button.
 */
export declare function buildDCFlow(providerLabel: string, recordCount: number, onAutoSetup: () => void, onManual: () => void): HTMLElement;
/**
 * Build the return URL for DC redirect.
 * Appends de_state=<domain> to the current page URL.
 */
export declare function buildReturnUrl(domain: string, baseReturnUrl?: string): string;
/**
 * Check if the page was loaded from a DC redirect return.
 * Returns the domain from de_state param, or null.
 */
export declare function checkDCReturn(): string | null;
/**
 * Clean the de_state param from the URL without reloading the page.
 */
export declare function cleanReturnUrl(): void;
