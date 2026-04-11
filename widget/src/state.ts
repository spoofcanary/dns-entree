/** Widget states. */
export const State = {
  IDLE: "idle",
  DETECTING: "detecting",
  DC_FLOW: "dc_flow",
  CREDENTIAL_FLOW: "credential_flow",
  FALLBACK: "fallback",
  PUSHING: "pushing",
  VERIFYING: "verifying",
  COMPLETE: "complete",
  ERROR: "error",
} as const;

export type StateValue = (typeof State)[keyof typeof State];

/** Events that trigger state transitions. */
export const Event = {
  OPEN: "OPEN",
  DETECTED: "DETECTED",
  DC_AVAILABLE: "DC_AVAILABLE",
  DC_RETURN: "DC_RETURN",
  CREDS_SUBMITTED: "CREDS_SUBMITTED",
  NO_CREDS: "NO_CREDS",
  PUSH_STARTED: "PUSH_STARTED",
  RECORD_VERIFIED: "RECORD_VERIFIED",
  ALL_VERIFIED: "ALL_VERIFIED",
  ERROR: "ERROR",
  CLOSE: "CLOSE",
} as const;

export type EventValue = (typeof Event)[keyof typeof Event];

/** Transition table. Pure function, no side effects. */
const transitions: Record<string, Partial<Record<EventValue, StateValue>>> = {
  [State.IDLE]: {
    [Event.OPEN]: State.DETECTING,
    [Event.DC_RETURN]: State.VERIFYING,
  },
  [State.DETECTING]: {
    [Event.DC_AVAILABLE]: State.DC_FLOW,
    [Event.DETECTED]: State.CREDENTIAL_FLOW,
    [Event.NO_CREDS]: State.FALLBACK,
    [Event.ERROR]: State.ERROR,
    [Event.CLOSE]: State.IDLE,
  },
  [State.DC_FLOW]: {
    [Event.PUSH_STARTED]: State.PUSHING,
    [Event.DC_RETURN]: State.VERIFYING,
    [Event.NO_CREDS]: State.FALLBACK,
    [Event.ERROR]: State.ERROR,
    [Event.CLOSE]: State.IDLE,
  },
  [State.CREDENTIAL_FLOW]: {
    [Event.CREDS_SUBMITTED]: State.PUSHING,
    [Event.NO_CREDS]: State.FALLBACK,
    [Event.ERROR]: State.ERROR,
    [Event.CLOSE]: State.IDLE,
  },
  [State.FALLBACK]: {
    [Event.PUSH_STARTED]: State.VERIFYING,
    [Event.ERROR]: State.ERROR,
    [Event.CLOSE]: State.IDLE,
  },
  [State.PUSHING]: {
    [Event.RECORD_VERIFIED]: State.VERIFYING,
    [Event.ALL_VERIFIED]: State.COMPLETE,
    [Event.ERROR]: State.ERROR,
    [Event.CLOSE]: State.IDLE,
  },
  [State.VERIFYING]: {
    [Event.ALL_VERIFIED]: State.COMPLETE,
    [Event.ERROR]: State.ERROR,
    [Event.CLOSE]: State.IDLE,
  },
  [State.COMPLETE]: {
    [Event.CLOSE]: State.IDLE,
  },
  [State.ERROR]: {
    [Event.CLOSE]: State.IDLE,
    [Event.OPEN]: State.DETECTING,
  },
};

/**
 * Pure state transition function.
 * Returns the new state, or the current state if the event is invalid.
 */
export function transition(current: StateValue, event: EventValue): StateValue {
  const next = transitions[current]?.[event];
  return next ?? current;
}
