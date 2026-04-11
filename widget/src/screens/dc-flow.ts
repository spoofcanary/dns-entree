import type { DCDiscoverData } from "../api";
import type { DnsEntreeOptions, DnsRecord } from "../types";

const ICON_BOLT = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>`;

const SESSION_KEY_PREFIX = "de_state_";

export interface DCFlowState {
  domain: string;
  records: DnsRecord[];
  dcDiscovery: DCDiscoverData;
  providerLabel: string;
  timestamp: number;
}

/** Save widget state to sessionStorage for resume after DC redirect. */
export function saveDCState(options: DnsEntreeOptions, dcDiscovery: DCDiscoverData, providerLabel: string): void {
  const state: DCFlowState = {
    domain: options.domain,
    records: options.records,
    dcDiscovery,
    providerLabel,
    timestamp: Date.now(),
  };
  try {
    sessionStorage.setItem(SESSION_KEY_PREFIX + options.domain, JSON.stringify(state));
  } catch {
    // sessionStorage unavailable or full - continue without persistence
  }
}

/** Restore saved state from sessionStorage. */
export function restoreDCState(domain: string): DCFlowState | null {
  try {
    const raw = sessionStorage.getItem(SESSION_KEY_PREFIX + domain);
    if (!raw) return null;
    const state = JSON.parse(raw) as DCFlowState;
    // Expire after 10 minutes
    if (Date.now() - state.timestamp > 10 * 60 * 1000) {
      sessionStorage.removeItem(SESSION_KEY_PREFIX + domain);
      return null;
    }
    return state;
  } catch {
    return null;
  }
}

/** Clean up saved state. */
export function clearDCState(domain: string): void {
  try {
    sessionStorage.removeItem(SESSION_KEY_PREFIX + domain);
  } catch {
    // ignore
  }
}

/**
 * Helper: safely insert a trusted, compile-time SVG literal into an element.
 * All SVG constants in this file are static string literals with zero user input.
 */
function setSvg(element: HTMLElement, svg: string): void {
  const tpl = document.createElement("template");
  tpl.innerHTML = svg;
  element.appendChild(tpl.content);
}

/**
 * Build the DC flow screen DOM.
 * Shows provider name and "Set up automatically" button.
 */
export function buildDCFlow(
  providerLabel: string,
  recordCount: number,
  onAutoSetup: () => void,
  onManual: () => void,
): HTMLElement {
  const container = document.createElement("div");
  container.className = "de-dc-flow";

  // Provider badge
  const badge = document.createElement("div");
  badge.className = "de-dc-badge";
  const boltIcon = document.createElement("span");
  boltIcon.className = "de-dc-bolt";
  setSvg(boltIcon, ICON_BOLT);
  badge.appendChild(boltIcon);
  const badgeText = document.createElement("span");
  badgeText.textContent = providerLabel + " supports one-click setup";
  badge.appendChild(badgeText);
  container.appendChild(badge);

  // Description
  const desc = document.createElement("p");
  desc.className = "de-dc-desc";
  desc.textContent = recordCount === 1
    ? "We can configure your DNS record automatically. You will be redirected to " + providerLabel + " to approve the changes."
    : "We can configure all " + recordCount + " DNS records automatically. You will be redirected to " + providerLabel + " to approve the changes.";
  container.appendChild(desc);

  // Record summary
  const summary = document.createElement("div");
  summary.className = "de-dc-summary";
  const summaryLabel = document.createElement("div");
  summaryLabel.className = "de-dc-summary-label";
  summaryLabel.textContent = recordCount + " record" + (recordCount !== 1 ? "s" : "") + " will be configured";
  summary.appendChild(summaryLabel);
  container.appendChild(summary);

  // Buttons
  const btnRow = document.createElement("div");
  btnRow.className = "de-dc-buttons";

  const autoBtn = document.createElement("button");
  autoBtn.className = "de-btn de-btn-primary de-dc-auto-btn";
  autoBtn.type = "button";
  const autoBolt = document.createElement("span");
  autoBolt.className = "de-dc-btn-bolt";
  setSvg(autoBolt, ICON_BOLT);
  autoBtn.appendChild(autoBolt);
  autoBtn.appendChild(document.createTextNode("Set up automatically"));
  autoBtn.addEventListener("click", onAutoSetup);
  btnRow.appendChild(autoBtn);

  const manualLink = document.createElement("button");
  manualLink.className = "de-btn de-btn-link";
  manualLink.type = "button";
  manualLink.textContent = "Set up manually instead";
  manualLink.addEventListener("click", onManual);
  btnRow.appendChild(manualLink);

  container.appendChild(btnRow);

  return container;
}

/**
 * Build the return URL for DC redirect.
 * Appends de_state=<domain> to the current page URL.
 */
export function buildReturnUrl(domain: string, baseReturnUrl?: string): string {
  const base = baseReturnUrl || window.location.href;
  const url = new URL(base);
  // Strip any existing de_state param
  url.searchParams.delete("de_state");
  url.searchParams.set("de_state", domain);
  return url.toString();
}

/**
 * Check if the page was loaded from a DC redirect return.
 * Returns the domain from de_state param, or null.
 */
export function checkDCReturn(): string | null {
  try {
    const params = new URLSearchParams(window.location.search);
    return params.get("de_state");
  } catch {
    return null;
  }
}

/**
 * Clean the de_state param from the URL without reloading the page.
 */
export function cleanReturnUrl(): void {
  try {
    const url = new URL(window.location.href);
    if (url.searchParams.has("de_state")) {
      url.searchParams.delete("de_state");
      window.history.replaceState({}, "", url.toString());
    }
  } catch {
    // ignore
  }
}
