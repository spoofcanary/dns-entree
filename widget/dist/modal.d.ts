import type { DnsEntreeOptions, RecordResult } from "./types";
/**
 * Modal controller. Manages shadow DOM content, focus trap,
 * keyboard handling, and state-driven rendering.
 */
export declare class Modal {
    private shadow;
    private overlay;
    private ctx;
    private previouslyFocused;
    private verifyPoller;
    private pushController;
    constructor(shadow: ShadowRoot, options: DnsEntreeOptions);
    /** Open the modal. Injects styles, renders overlay, starts detecting. */
    open(): void;
    /**
     * Open the modal directly into verifying state (DC return flow).
     * Called when the page loads with de_state param after a DC redirect.
     */
    openForVerify(records: RecordResult[], providerLabel: string): void;
    /** Close the modal. Cleans up DOM and restores focus. */
    close(): void;
    /** Dispatch an event through the state machine and re-render. */
    private dispatch;
    /** Run real provider detection via the API. */
    private startDetection;
    /** Handle "Set up automatically" button in DC flow. */
    private handleDCAutoSetup;
    /** Handle "Set up manually instead" in DC flow. */
    private handleDCManual;
    /** Start verification polling. */
    private startVerifyPolling;
    /** Stop verification polling. */
    private stopVerifyPolling;
    /** Manual re-verify after timeout. */
    private handleManualVerify;
    /** Re-render the modal content based on current state. */
    private render;
    /** Build the full card element. */
    private buildCard;
    /** Build the header with title and close button. */
    private buildHeader;
    /** Build body content based on current state. */
    private buildBody;
    private buildFallback;
    /** Build a single record card for fallback view with type badge, name, value, and copy. */
    private buildRecordCard;
    /** Build the "Add records manually" link used in DC and credential screens. */
    private buildManualFallbackLink;
    private buildComplete;
    private buildError;
    /** Build footer with action buttons based on state. */
    private buildFooter;
    /** Copy record value to clipboard. */
    private handleCopy;
    /** Handle credential form submission. */
    private handleCredentialSubmit;
    /** Handle "I don't have API access" from credential flow. */
    private handleCredentialNoCreds;
    /** Handle retry after push error. Goes back to credential form. */
    private handlePushRetry;
    /** Stop any active push controller. */
    private stopPushController;
    /** Start real verification via API polling. */
    private handleVerify;
    /** Focus first focusable element. */
    private focusFirst;
    /** Keyboard event handler. */
    private handleKeydown;
    /** Keep focus cycling within the modal. */
    private trapFocus;
}
