import { verify } from "../api";
import type { ApiConfig } from "../api";
import type { DnsEntreeOptions, RecordResult } from "../types";

const VERIFY_INTERVAL_MS = 3000;
const VERIFY_TIMEOUT_MS = 2 * 60 * 1000;

const ICON_CHECK = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`;

/**
 * Safe SVG insertion from compile-time string literals only.
 * All SVG constants in this module are static with no user input.
 */
function setSvg(element: HTMLElement, svg: string): void {
  const tpl = document.createElement("template");
  tpl.innerHTML = svg;
  element.appendChild(tpl.content);
}

export interface VerifyPoller {
  stop: () => void;
}

/**
 * Start polling /v1/verify for each record.
 * Calls onRecordVerified when a record passes, onAllVerified when all pass,
 * onTimeout after 2 minutes.
 */
export function startVerifyPolling(
  options: DnsEntreeOptions,
  results: RecordResult[],
  callbacks: {
    onRecordVerified: (index: number) => void;
    onAllVerified: () => void;
    onTimeout: () => void;
  },
): VerifyPoller {
  const cfg: ApiConfig = { apiUrl: options.apiUrl, apiKey: options.apiKey };
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | null = null;
  const startTime = Date.now();

  async function poll(): Promise<void> {
    if (stopped) return;

    const pending = results
      .map((r, i) => ({ r, i }))
      .filter((x) => x.r.status !== "verified");

    if (pending.length === 0) {
      callbacks.onAllVerified();
      return;
    }

    // Check timeout
    if (Date.now() - startTime > VERIFY_TIMEOUT_MS) {
      callbacks.onTimeout();
      return;
    }

    // Verify all pending records in parallel
    const checks = pending.map(async ({ r, i }) => {
      const res = await verify(
        cfg,
        options.domain,
        r.record.type,
        r.record.name,
        r.record.content,
      );
      if (res.ok && res.data.verified) {
        results[i].status = "verified";
        callbacks.onRecordVerified(i);
      }
    });

    await Promise.all(checks);

    // Check if all done after this round
    if (results.every((r) => r.status === "verified")) {
      callbacks.onAllVerified();
      return;
    }

    // Schedule next poll
    if (!stopped) {
      timer = setTimeout(poll, VERIFY_INTERVAL_MS);
    }
  }

  // Start first poll
  poll();

  return {
    stop() {
      stopped = true;
      if (timer !== null) {
        clearTimeout(timer);
        timer = null;
      }
    },
  };
}

/** Build the verifying screen DOM with per-record checklist. */
export function buildVerifying(
  results: RecordResult[],
  timedOut: boolean,
  onManualVerify?: () => void,
): HTMLElement {
  const container = document.createElement("div");

  const intro = document.createElement("p");
  intro.style.cssText = "margin-bottom: 16px; color: var(--de-fg-muted); font-size: 13px;";
  if (timedOut) {
    intro.textContent = "DNS changes can take a few minutes to propagate.";
  } else {
    intro.textContent = "Checking DNS propagation...";
  }
  container.appendChild(intro);

  const list = document.createElement("div");
  list.className = "de-records-list";

  for (const result of results) {
    list.appendChild(buildVerifyItem(result));
  }

  container.appendChild(list);

  if (timedOut && onManualVerify) {
    const btnWrap = document.createElement("div");
    btnWrap.style.cssText = "margin-top: 16px; text-align: center;";
    const retryBtn = document.createElement("button");
    retryBtn.className = "de-btn de-btn-primary";
    retryBtn.type = "button";
    retryBtn.textContent = "Check again";
    retryBtn.addEventListener("click", onManualVerify);
    btnWrap.appendChild(retryBtn);
    container.appendChild(btnWrap);
  }

  return container;
}

function buildVerifyItem(result: RecordResult): HTMLElement {
  const r = result.record;
  const item = document.createElement("div");
  item.className = "de-record-item de-verify-item";

  // Header: type badge + name
  const header = document.createElement("div");
  header.className = "de-record-header";
  const typeBadge = document.createElement("span");
  typeBadge.className = "de-record-type";
  typeBadge.textContent = r.type;
  header.appendChild(typeBadge);
  const nameSpan = document.createElement("span");
  nameSpan.className = "de-record-name";
  nameSpan.textContent = r.name;
  header.appendChild(nameSpan);
  item.appendChild(header);

  // Status row with icon
  const statusRow = document.createElement("div");
  statusRow.className = "de-verify-status";

  if (result.status === "verified") {
    const checkIcon = document.createElement("span");
    checkIcon.className = "de-verify-check";
    setSvg(checkIcon, ICON_CHECK);
    statusRow.appendChild(checkIcon);
    const label = document.createElement("span");
    label.className = "de-status-text de-verified";
    label.textContent = "Verified";
    statusRow.appendChild(label);
  } else {
    const dot = document.createElement("span");
    dot.className = "de-status-dot de-pushing";
    statusRow.appendChild(dot);
    const label = document.createElement("span");
    label.className = "de-status-text";
    label.textContent = "Waiting for propagation...";
    statusRow.appendChild(label);
  }

  item.appendChild(statusRow);

  return item;
}
