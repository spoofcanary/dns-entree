import type { DnsEntreeOptions, RecordResult } from "../types";
export interface FormField {
    key: string;
    label: string;
    placeholder: string;
    secret: boolean;
    /** Header name sent to entree-api */
    header: string;
}
export interface ProviderForm {
    slug: string;
    label: string;
    fields: FormField[];
    helpUrl: string;
    helpText: string;
}
/** Look up the form definition for a provider slug. */
export declare function getProviderForm(provider: string): ProviderForm;
export interface CredentialFlowCallbacks {
    onSubmit: (provider: string, creds: Record<string, string>) => void;
    onNoCreds: () => void;
}
/**
 * Build the credential flow screen: provider form, security notice, help section.
 * Returns the container element. Field values are tracked internally.
 */
export declare function buildCredentialFlow(provider: string, callbacks: CredentialFlowCallbacks): HTMLElement;
export interface PushCallbacks {
    onRender: () => void;
    onAllDone: () => void;
    onError: (message: string) => void;
}
export interface PushController {
    stop: () => void;
}
/**
 * Push records via /v1/apply, then poll /v1/verify for each.
 * Updates results in-place and calls onRender after each status change.
 */
export declare function startPushFlow(options: DnsEntreeOptions, provider: string, creds: Record<string, string>, results: RecordResult[], callbacks: PushCallbacks): PushController;
/**
 * Build the pushing/verifying screen with per-record status.
 * Reuses the same visual pattern as verifying.ts but adds error handling.
 */
export declare function buildPushProgress(results: RecordResult[], errorMessage: string | null, onRetry: (() => void) | null): HTMLElement;
