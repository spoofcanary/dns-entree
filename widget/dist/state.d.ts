/** Widget states. */
export declare const State: {
    readonly IDLE: "idle";
    readonly DETECTING: "detecting";
    readonly DC_FLOW: "dc_flow";
    readonly CREDENTIAL_FLOW: "credential_flow";
    readonly FALLBACK: "fallback";
    readonly PUSHING: "pushing";
    readonly VERIFYING: "verifying";
    readonly COMPLETE: "complete";
    readonly ERROR: "error";
};
export type StateValue = (typeof State)[keyof typeof State];
/** Events that trigger state transitions. */
export declare const Event: {
    readonly OPEN: "OPEN";
    readonly DETECTED: "DETECTED";
    readonly DC_AVAILABLE: "DC_AVAILABLE";
    readonly DC_RETURN: "DC_RETURN";
    readonly CREDS_SUBMITTED: "CREDS_SUBMITTED";
    readonly NO_CREDS: "NO_CREDS";
    readonly PUSH_STARTED: "PUSH_STARTED";
    readonly RECORD_VERIFIED: "RECORD_VERIFIED";
    readonly ALL_VERIFIED: "ALL_VERIFIED";
    readonly ERROR: "ERROR";
    readonly CLOSE: "CLOSE";
};
export type EventValue = (typeof Event)[keyof typeof Event];
/**
 * Pure state transition function.
 * Returns the new state, or the current state if the event is invalid.
 */
export declare function transition(current: StateValue, event: EventValue): StateValue;
