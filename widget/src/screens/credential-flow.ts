import { apply, verify } from "../api";
import type { ApiConfig, ApplyData } from "../api";
import type { DnsEntreeOptions, RecordResult } from "../types";

// --- Provider form definitions ---

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

const PROVIDER_FORMS: Record<string, ProviderForm> = {
  cloudflare: {
    slug: "cloudflare",
    label: "Cloudflare",
    fields: [
      {
        key: "api_token",
        label: "API Token",
        placeholder: "Paste your Cloudflare API token",
        secret: true,
        header: "X-Entree-Cloudflare-Token",
      },
    ],
    helpUrl: "https://developers.cloudflare.com/fundamentals/api/get-started/create-token/",
    helpText:
      "Create a Custom Token with Zone > DNS > Edit permission. " +
      "Select the specific zone (domain) you want to manage.",
  },
  route53: {
    slug: "route53",
    label: "Amazon Route 53",
    fields: [
      {
        key: "access_key_id",
        label: "Access Key ID",
        placeholder: "AKIA...",
        secret: false,
        header: "X-Entree-AWS-Access-Key-Id",
      },
      {
        key: "secret_access_key",
        label: "Secret Access Key",
        placeholder: "Paste your secret access key",
        secret: true,
        header: "X-Entree-AWS-Secret-Access-Key",
      },
    ],
    helpUrl: "https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_access-keys.html",
    helpText:
      "Create an IAM user with AmazonRoute53FullAccess policy, " +
      "then generate an access key pair in the Security Credentials tab.",
  },
  godaddy: {
    slug: "godaddy",
    label: "GoDaddy",
    fields: [
      {
        key: "api_key",
        label: "API Key",
        placeholder: "Paste your API key",
        secret: false,
        header: "X-Entree-GoDaddy-Key",
      },
      {
        key: "api_secret",
        label: "API Secret",
        placeholder: "Paste your API secret",
        secret: true,
        header: "X-Entree-GoDaddy-Secret",
      },
    ],
    helpUrl: "https://developer.godaddy.com/keys",
    helpText:
      "Go to the GoDaddy Developer Portal and create a Production API key. " +
      "Note: API access requires a paid plan or Discount Domain Club membership.",
  },
  google_cloud_dns: {
    slug: "google_cloud_dns",
    label: "Google Cloud DNS",
    fields: [
      {
        key: "service_account_json",
        label: "Service Account JSON",
        placeholder: "Paste the full JSON key file contents",
        secret: true,
        header: "X-Entree-GCDNS-Service-Account-JSON",
      },
      {
        key: "project_id",
        label: "Project ID",
        placeholder: "my-project-123",
        secret: false,
        header: "X-Entree-GCDNS-Project-Id",
      },
    ],
    helpUrl: "https://cloud.google.com/iam/docs/keys-create-delete",
    helpText:
      "Create a service account with DNS Administrator role, " +
      "then create and download a JSON key for that account.",
  },
};

const GENERIC_FORM: ProviderForm = {
  slug: "generic",
  label: "DNS Provider",
  fields: [
    {
      key: "api_token",
      label: "Provider API Token",
      placeholder: "Paste your API token",
      secret: true,
      header: "X-Entree-Cloudflare-Token",
    },
  ],
  helpUrl: "",
  helpText: "Enter the API token for your DNS provider.",
};

/** Look up the form definition for a provider slug. */
export function getProviderForm(provider: string): ProviderForm {
  return PROVIDER_FORMS[provider] || { ...GENERIC_FORM, label: provider || "DNS Provider" };
}

// --- SVG icons (static compile-time literals, no user input) ---

const ICON_LOCK = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>`;
const ICON_EYE = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>`;
const ICON_EYE_OFF = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>`;
const ICON_CHEVRON = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>`;
const ICON_CHECK = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`;
const ICON_X = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`;

/**
 * Set a trusted, compile-time SVG literal on an element.
 * All SVG constants in this file are static string literals with zero user input.
 * This is the same pattern used in modal.ts, dc-flow.ts, and verifying.ts.
 */
function setSvg(element: HTMLElement, svg: string): void {
  const tpl = document.createElement("template");
  tpl.innerHTML = svg;
  element.appendChild(tpl.content);
}

// --- Credential flow screen builder ---

export interface CredentialFlowCallbacks {
  onSubmit: (provider: string, creds: Record<string, string>) => void;
  onNoCreds: () => void;
}

/**
 * Build the credential flow screen: provider form, security notice, help section.
 * Returns the container element. Field values are tracked internally.
 */
export function buildCredentialFlow(
  provider: string,
  callbacks: CredentialFlowCallbacks,
): HTMLElement {
  const form = getProviderForm(provider);
  const container = document.createElement("div");
  container.className = "de-cred-flow";

  // Provider label
  const providerRow = document.createElement("div");
  providerRow.className = "de-cred-provider";
  providerRow.textContent = form.label;
  container.appendChild(providerRow);

  // Field values storage
  const values: Record<string, string> = {};
  const inputs: HTMLInputElement[] = [];

  // Form fields
  const fieldsWrap = document.createElement("div");
  fieldsWrap.className = "de-cred-fields";

  for (const field of form.fields) {
    values[field.key] = "";
    const group = document.createElement("div");
    group.className = "de-cred-field-group";

    const label = document.createElement("label");
    label.className = "de-cred-label";
    label.textContent = field.label;
    const inputId = "de-cred-" + field.key;
    label.setAttribute("for", inputId);
    group.appendChild(label);

    const inputWrap = document.createElement("div");
    inputWrap.className = "de-cred-input-wrap";

    const input = document.createElement("input");
    input.type = field.secret ? "password" : "text";
    input.id = inputId;
    input.className = "de-cred-input";
    input.placeholder = field.placeholder;
    input.autocomplete = "off";
    input.spellcheck = false;
    input.setAttribute("data-field-key", field.key);
    input.addEventListener("input", () => {
      values[field.key] = input.value;
      updateSubmitState();
    });
    inputs.push(input);
    inputWrap.appendChild(input);

    // Show/hide toggle for secret fields
    if (field.secret) {
      const toggle = document.createElement("button");
      toggle.type = "button";
      toggle.className = "de-cred-toggle";
      toggle.setAttribute("aria-label", "Show password");
      toggle.tabIndex = -1;
      setSvg(toggle, ICON_EYE);
      let visible = false;
      toggle.addEventListener("click", () => {
        visible = !visible;
        input.type = visible ? "text" : "password";
        toggle.setAttribute("aria-label", visible ? "Hide password" : "Show password");
        while (toggle.firstChild) toggle.removeChild(toggle.firstChild);
        setSvg(toggle, visible ? ICON_EYE_OFF : ICON_EYE);
      });
      inputWrap.appendChild(toggle);
    }

    group.appendChild(inputWrap);
    fieldsWrap.appendChild(group);
  }

  container.appendChild(fieldsWrap);

  // Security notice
  const secNotice = document.createElement("div");
  secNotice.className = "de-cred-security";
  const lockIcon = document.createElement("span");
  lockIcon.className = "de-cred-lock-icon";
  setSvg(lockIcon, ICON_LOCK);
  secNotice.appendChild(lockIcon);
  const secText = document.createElement("span");
  secText.textContent = "Credentials are sent directly to your DNS provider's API. They are not stored.";
  secNotice.appendChild(secText);
  container.appendChild(secNotice);

  // Help expandable section
  if (form.helpText) {
    const details = document.createElement("div");
    details.className = "de-cred-help";
    const helpBtn = document.createElement("button");
    helpBtn.type = "button";
    helpBtn.className = "de-cred-help-toggle";
    helpBtn.textContent = "How to get these credentials";
    const chevron = document.createElement("span");
    chevron.className = "de-cred-help-chevron";
    setSvg(chevron, ICON_CHEVRON);
    helpBtn.appendChild(chevron);

    const helpBody = document.createElement("div");
    helpBody.className = "de-cred-help-body";
    helpBody.style.display = "none";

    const helpP = document.createElement("p");
    helpP.textContent = form.helpText;
    helpBody.appendChild(helpP);

    if (form.helpUrl) {
      const link = document.createElement("a");
      link.href = form.helpUrl;
      link.target = "_blank";
      link.rel = "noopener noreferrer";
      link.className = "de-cred-help-link";
      link.textContent = "View documentation";
      helpBody.appendChild(link);
    }

    let expanded = false;
    helpBtn.addEventListener("click", () => {
      expanded = !expanded;
      helpBody.style.display = expanded ? "block" : "none";
      chevron.classList.toggle("de-expanded", expanded);
      helpBtn.setAttribute("aria-expanded", String(expanded));
    });

    details.appendChild(helpBtn);
    details.appendChild(helpBody);
    container.appendChild(details);
  }

  // Action buttons
  const actions = document.createElement("div");
  actions.className = "de-cred-actions";

  const submitBtn = document.createElement("button");
  submitBtn.type = "button";
  submitBtn.className = "de-btn de-btn-primary de-cred-submit";
  submitBtn.textContent = "Push records";
  submitBtn.disabled = true;
  submitBtn.addEventListener("click", () => {
    // Build credential headers map
    const creds: Record<string, string> = {};
    for (const field of form.fields) {
      let val = values[field.key];
      // Google Cloud DNS: base64-encode the JSON
      if (field.header === "X-Entree-GCDNS-Service-Account-JSON" && val) {
        try {
          val = btoa(val);
        } catch {
          // If btoa fails (non-latin chars), use manual encoding
          val = btoa(unescape(encodeURIComponent(val)));
        }
      }
      creds[field.header] = val;
    }
    callbacks.onSubmit(form.slug === "generic" ? provider : form.slug, creds);
  });
  actions.appendChild(submitBtn);

  const noCredsLink = document.createElement("button");
  noCredsLink.type = "button";
  noCredsLink.className = "de-btn de-btn-link";
  noCredsLink.textContent = "I don't have API access";
  noCredsLink.addEventListener("click", callbacks.onNoCreds);
  actions.appendChild(noCredsLink);

  container.appendChild(actions);

  function updateSubmitState(): void {
    const allFilled = form.fields.every((f) => values[f.key].trim().length > 0);
    submitBtn.disabled = !allFilled;
  }

  return container;
}

// --- Push flow: apply records + per-record verify ---

const VERIFY_INTERVAL_MS = 3000;
const VERIFY_TIMEOUT_MS = 2 * 60 * 1000;

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
export function startPushFlow(
  options: DnsEntreeOptions,
  provider: string,
  creds: Record<string, string>,
  results: RecordResult[],
  callbacks: PushCallbacks,
): PushController {
  const cfg: ApiConfig = { apiUrl: options.apiUrl, apiKey: options.apiKey };
  let stopped = false;
  let verifyTimer: ReturnType<typeof setTimeout> | null = null;

  // Mark all as pushing
  for (const r of results) {
    r.status = "pushing";
  }
  callbacks.onRender();

  // Call apply
  apply(cfg, provider, creds, options.domain, options.records)
    .then((res) => {
      if (stopped) return;

      if (!res.ok) {
        // Map error codes to friendly messages
        const msg = mapApplyError(res.error, res.code, provider);
        callbacks.onError(msg);
        return;
      }

      // Match apply results to our records
      const data = res.data as ApplyData;
      for (let i = 0; i < results.length; i++) {
        const match = data.results.find(
          (ar) =>
            ar.type === results[i].record.type &&
            ar.name === results[i].record.name,
        );
        if (match) {
          if (match.status === "error" || match.status === "failed") {
            results[i].status = "failed";
            results[i].error = match.verify_error || "Failed to apply record";
          } else if (match.verified) {
            results[i].status = "verified";
          } else {
            // Applied but not yet verified - start polling
            results[i].status = "pushing";
          }
        }
      }

      // Clear credentials from memory
      for (const k of Object.keys(creds)) {
        creds[k] = "";
      }

      callbacks.onRender();

      // Check if all already done
      if (results.every((r) => r.status === "verified" || r.status === "failed")) {
        if (results.some((r) => r.status === "verified")) {
          callbacks.onAllDone();
        }
        return;
      }

      // Start verify polling for records still pending
      startVerifyLoop();
    })
    .catch((err) => {
      if (stopped) return;
      callbacks.onError(
        err instanceof Error ? err.message : "Cannot reach DNS service",
      );
    });

  function startVerifyLoop(): void {
    const startTime = Date.now();

    async function poll(): Promise<void> {
      if (stopped) return;

      if (Date.now() - startTime > VERIFY_TIMEOUT_MS) {
        // Mark remaining as-is, let the UI handle partial state
        callbacks.onRender();
        callbacks.onAllDone();
        return;
      }

      const pending = results.filter(
        (r) => r.status === "pushing",
      );

      if (pending.length === 0) {
        callbacks.onAllDone();
        return;
      }

      const checks = pending.map(async (r) => {
        const res = await verify(
          cfg,
          options.domain,
          r.record.type,
          r.record.name,
          r.record.content,
        );
        if (res.ok && res.data.verified) {
          r.status = "verified";
          callbacks.onRender();
        }
      });

      await Promise.all(checks);

      if (results.every((r) => r.status === "verified" || r.status === "failed")) {
        callbacks.onAllDone();
        return;
      }

      if (!stopped) {
        verifyTimer = setTimeout(poll, VERIFY_INTERVAL_MS);
      }
    }

    poll();
  }

  return {
    stop() {
      stopped = true;
      if (verifyTimer !== null) {
        clearTimeout(verifyTimer);
        verifyTimer = null;
      }
    },
  };
}

function mapApplyError(
  message: string,
  code: string | undefined,
  provider: string,
): string {
  if (code === "missing_credentials" || code === "invalid_credentials") {
    switch (provider) {
      case "cloudflare":
        return "Invalid API token. Check that your token has Zone > DNS > Edit permission.";
      case "route53":
        return "Invalid AWS credentials. Verify your Access Key ID and Secret Access Key.";
      case "godaddy":
        return "Invalid GoDaddy API credentials. Check your key and secret, and ensure API access is enabled.";
      case "google_cloud_dns":
        return "Invalid service account JSON. Ensure the account has DNS Administrator role.";
      default:
        return "Invalid credentials. Check your API token and try again.";
    }
  }
  if (code === "rate_limited") {
    return "Rate limit reached. Please wait a moment and try again.";
  }
  if (code === "zone_not_found") {
    return "Domain zone not found. Check that this domain exists in your " + provider + " account.";
  }
  if (message.includes("Network error") || message.includes("fetch")) {
    return "Cannot reach DNS service. Check your connection and try again.";
  }
  return message || "Failed to apply records. Please try again.";
}

// --- Pushing screen: per-record progress checklist ---

/**
 * Build the pushing/verifying screen with per-record status.
 * Reuses the same visual pattern as verifying.ts but adds error handling.
 */
export function buildPushProgress(
  results: RecordResult[],
  errorMessage: string | null,
  onRetry: (() => void) | null,
): HTMLElement {
  const container = document.createElement("div");

  const intro = document.createElement("p");
  intro.style.cssText = "margin-bottom: 16px; color: var(--de-fg-muted); font-size: 13px;";
  if (errorMessage) {
    intro.textContent = errorMessage;
    intro.style.color = "var(--de-error)";
  } else {
    const allVerified = results.every((r) => r.status === "verified");
    const anyFailed = results.some((r) => r.status === "failed");
    if (allVerified) {
      intro.textContent = "All records applied and verified.";
    } else if (anyFailed) {
      const ok = results.filter((r) => r.status === "verified").length;
      intro.textContent = ok + " of " + results.length + " records applied.";
    } else {
      intro.textContent = "Pushing records to your DNS provider...";
    }
  }
  container.appendChild(intro);

  const list = document.createElement("div");
  list.className = "de-records-list";

  for (const result of results) {
    list.appendChild(buildPushItem(result));
  }
  container.appendChild(list);

  if (errorMessage && onRetry) {
    const btnWrap = document.createElement("div");
    btnWrap.style.cssText = "margin-top: 16px; text-align: center;";
    const retryBtn = document.createElement("button");
    retryBtn.className = "de-btn de-btn-primary";
    retryBtn.type = "button";
    retryBtn.textContent = "Try again";
    retryBtn.addEventListener("click", onRetry);
    btnWrap.appendChild(retryBtn);
    container.appendChild(btnWrap);
  }

  return container;
}

function buildPushItem(result: RecordResult): HTMLElement {
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

  // Status row
  const statusRow = document.createElement("div");
  statusRow.className = "de-verify-status";

  switch (result.status) {
    case "verified": {
      const checkIcon = document.createElement("span");
      checkIcon.className = "de-verify-check";
      setSvg(checkIcon, ICON_CHECK);
      statusRow.appendChild(checkIcon);
      const label = document.createElement("span");
      label.className = "de-status-text de-verified";
      label.textContent = "Verified";
      statusRow.appendChild(label);
      break;
    }
    case "failed": {
      const failIcon = document.createElement("span");
      failIcon.className = "de-verify-fail";
      setSvg(failIcon, ICON_X);
      statusRow.appendChild(failIcon);
      const label = document.createElement("span");
      label.className = "de-status-text de-failed";
      label.textContent = result.error || "Failed";
      statusRow.appendChild(label);
      break;
    }
    case "pushing": {
      const dot = document.createElement("span");
      dot.className = "de-status-dot de-pushing";
      statusRow.appendChild(dot);
      const label = document.createElement("span");
      label.className = "de-status-text";
      label.textContent = "Pushing...";
      statusRow.appendChild(label);
      break;
    }
    default: {
      const dot = document.createElement("span");
      dot.className = "de-status-dot de-pending";
      statusRow.appendChild(dot);
      const label = document.createElement("span");
      label.className = "de-status-text";
      label.textContent = "Pending";
      statusRow.appendChild(label);
      break;
    }
  }

  item.appendChild(statusRow);
  return item;
}
