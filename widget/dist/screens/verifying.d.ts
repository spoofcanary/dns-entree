import type { DnsEntreeOptions, RecordResult } from "../types";
export interface VerifyPoller {
    stop: () => void;
}
/**
 * Start polling /v1/verify for each record.
 * Calls onRecordVerified when a record passes, onAllVerified when all pass,
 * onTimeout after 2 minutes.
 */
export declare function startVerifyPolling(options: DnsEntreeOptions, results: RecordResult[], callbacks: {
    onRecordVerified: (index: number) => void;
    onAllVerified: () => void;
    onTimeout: () => void;
}): VerifyPoller;
/** Build the verifying screen DOM with per-record checklist. */
export declare function buildVerifying(results: RecordResult[], timedOut: boolean, onManualVerify?: () => void): HTMLElement;
