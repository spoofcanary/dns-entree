import { styles } from "./styles";
import { State, Event, transition, type StateValue, type EventValue } from "./state";
import { resolveTheme, getThemeVars, applyThemeVars } from "./theme";
import type { DnsEntreeOptions, RecordResult } from "./types";
import { dcApplyUrl, type ApiConfig, type DCDiscoverData } from "./api";
import { buildDetecting, runDetection, type DetectionOutcome } from "./screens/detecting";
import {
  buildDCFlow,
  saveDCState,
  buildReturnUrl,
  clearDCState,
} from "./screens/dc-flow";
import {
  buildVerifying,
  startVerifyPolling,
  type VerifyPoller,
} from "./screens/verifying";
import {
  buildCredentialFlow,
  buildPushProgress,
  startPushFlow,
  type PushController,
} from "./screens/credential-flow";

/** SVG icon helpers - inline to avoid external deps. */
const ICON_CLOSE = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`;
const ICON_COPY = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>`;
const ICON_CHECK = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`;
const ICON_ALERT = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>`;

/** Provider DNS management page URLs for fallback instructions. */
const PROVIDER_DNS_URLS: Record<string, string> = {
  cloudflare: "https://dash.cloudflare.com/?to=/:account/:zone/dns/records",
  route53: "https://console.aws.amazon.com/route53/v2/hostedzones",
  godaddy: "https://dcc.godaddy.com/manage/dns",
  google_cloud_dns: "https://console.cloud.google.com/net-services/dns/zones",
};

interface ModalContext {
  options: DnsEntreeOptions;
  state: StateValue;
  results: RecordResult[];
  errorMessage: string;
  detectionOutcome: DetectionOutcome | null;
  dcDiscovery: DCDiscoverData | null;
  providerLabel: string;
  providerSlug: string;
  verifyTimedOut: boolean;
  pushErrorMessage: string;
}

// Helper: create element with attributes
function el(tag: string, attrs?: Record<string, string>, children?: (Node | string)[]): HTMLElement {
  const node = document.createElement(tag);
  if (attrs) {
    for (const [k, v] of Object.entries(attrs)) {
      if (k === "className") {
        node.className = v;
      } else {
        node.setAttribute(k, v);
      }
    }
  }
  if (children) {
    for (const c of children) {
      if (typeof c === "string") {
        node.appendChild(document.createTextNode(c));
      } else {
        node.appendChild(c);
      }
    }
  }
  return node;
}

/**
 * Set a trusted, compile-time SVG literal on an element.
 * All SVG constants in this file are static string literals with zero user input.
 */
function setSvg(element: HTMLElement, svg: string): void {
  const template = document.createElement("template");
  template.innerHTML = svg;
  const content = template.content;
  element.appendChild(content);
}

/**
 * Modal controller. Manages shadow DOM content, focus trap,
 * keyboard handling, and state-driven rendering.
 */
export class Modal {
  private shadow: ShadowRoot;
  private overlay: HTMLDivElement | null = null;
  private ctx: ModalContext;
  private previouslyFocused: Element | null = null;
  private verifyPoller: VerifyPoller | null = null;
  private pushController: PushController | null = null;

  constructor(shadow: ShadowRoot, options: DnsEntreeOptions) {
    this.shadow = shadow;
    this.ctx = {
      options,
      state: State.IDLE,
      results: options.records.map((r) => ({
        record: r,
        status: "pending" as const,
      })),
      errorMessage: "",
      detectionOutcome: null,
      dcDiscovery: null,
      providerLabel: "",
      providerSlug: "",
      verifyTimedOut: false,
      pushErrorMessage: "",
    };
  }

  /** Open the modal. Injects styles, renders overlay, starts detecting. */
  open(): void {
    this.previouslyFocused = document.activeElement;

    // Inject styles
    const styleEl = document.createElement("style");
    styleEl.textContent = styles;
    this.shadow.appendChild(styleEl);

    // Apply theme
    const resolved = resolveTheme(this.ctx.options.theme);
    const vars = getThemeVars(resolved, this.ctx.options.accentColor);
    applyThemeVars(this.shadow.host as HTMLElement, vars);

    // Transition to detecting
    this.dispatch(Event.OPEN);

    // Render
    this.render();

    // Show with animation - request next frame so transition fires
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        this.overlay?.classList.add("de-visible");
      });
    });

    // Keyboard handler
    this.shadow.host.addEventListener("keydown", this.handleKeydown as EventListener);

    // Start real provider detection
    this.startDetection();
  }

  /**
   * Open the modal directly into verifying state (DC return flow).
   * Called when the page loads with de_state param after a DC redirect.
   */
  openForVerify(records: RecordResult[], providerLabel: string): void {
    this.previouslyFocused = document.activeElement;

    // Override results with the restored records
    this.ctx.results = records;
    this.ctx.providerLabel = providerLabel;

    // Inject styles
    const styleEl = document.createElement("style");
    styleEl.textContent = styles;
    this.shadow.appendChild(styleEl);

    // Apply theme
    const resolved = resolveTheme(this.ctx.options.theme);
    const vars = getThemeVars(resolved, this.ctx.options.accentColor);
    applyThemeVars(this.shadow.host as HTMLElement, vars);

    // Jump straight to verifying
    this.dispatch(Event.DC_RETURN);

    // Render
    this.render();

    // Show with animation
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        this.overlay?.classList.add("de-visible");
      });
    });

    // Keyboard handler
    this.shadow.host.addEventListener("keydown", this.handleKeydown as EventListener);

    // Start verification polling
    this.startVerifyPolling();
  }

  /** Close the modal. Cleans up DOM and restores focus. */
  close(): void {
    if (!this.overlay) return;

    // Stop any active polling/pushing
    this.stopVerifyPolling();
    this.stopPushController();

    this.overlay.classList.remove("de-visible");

    const onEnd = () => {
      this.shadow.host.removeEventListener("keydown", this.handleKeydown as EventListener);
      while (this.shadow.firstChild) {
        this.shadow.removeChild(this.shadow.firstChild);
      }
      this.overlay = null;

      if (this.previouslyFocused && this.previouslyFocused instanceof HTMLElement) {
        this.previouslyFocused.focus();
      }

      this.ctx.options.onClose?.();
    };

    this.overlay.addEventListener("transitionend", onEnd, { once: true });
    setTimeout(onEnd, 300);
  }

  /** Dispatch an event through the state machine and re-render. */
  private dispatch(event: EventValue): void {
    this.ctx.state = transition(this.ctx.state, event);
  }

  /** Run real provider detection via the API. */
  private async startDetection(): Promise<void> {
    try {
      const outcome = await runDetection(this.ctx.options);
      this.ctx.detectionOutcome = outcome;

      switch (outcome.type) {
        case "dc":
          this.ctx.dcDiscovery = outcome.dcDiscovery || null;
          this.ctx.providerLabel = outcome.label || outcome.provider || "your provider";
          this.dispatch(Event.DC_AVAILABLE);
          break;
        case "credential":
          this.ctx.providerLabel = outcome.label || outcome.provider || "";
          this.ctx.providerSlug = outcome.provider || "";
          this.dispatch(Event.DETECTED);
          break;
        case "fallback":
        default:
          this.ctx.providerLabel = outcome.label || outcome.provider || "";
          this.dispatch(Event.NO_CREDS);
          break;
      }
    } catch (err) {
      // API unreachable - graceful fallback to copy-paste (D-20)
      this.dispatch(Event.NO_CREDS);
    }

    this.render();
  }

  /** Handle "Set up automatically" button in DC flow. */
  private async handleDCAutoSetup(): Promise<void> {
    if (!this.ctx.dcDiscovery) {
      // No DC discovery data - fall through to manual
      this.dispatch(Event.NO_CREDS);
      this.render();
      return;
    }

    // Save state for resume after redirect
    saveDCState(this.ctx.options, this.ctx.dcDiscovery, this.ctx.providerLabel);

    // Build the return URL
    const returnUrl = buildReturnUrl(this.ctx.options.domain, this.ctx.options.returnUrl);

    // Build signed apply URL server-side via /v1/dc/apply-url.
    // The server holds the signing key and provider config.
    const cfg: ApiConfig = { apiUrl: this.ctx.options.apiUrl, apiKey: this.ctx.options.apiKey };
    const dc = this.ctx.dcDiscovery;
    if (!dc) {
      this.ctx.errorMessage = "Domain Connect discovery data missing.";
      this.dispatch(Event.ERROR);
      this.render();
      return;
    }

    const applyResult = await dcApplyUrl(cfg, {
      domain: this.ctx.options.domain,
      provider_id: dc.ProviderID,
      url_async_ux: dc.URLAsyncUX || dc.URLSyncUX,
      redirect_uri: returnUrl,
    });

    if (!applyResult.ok) {
      this.dispatch(Event.ERROR);
      this.ctx.errorMessage = applyResult.error;
      this.render();
      return;
    }

    window.location.href = applyResult.data.url;
  }

  /** Handle "Set up manually instead" in DC flow. */
  private handleDCManual(): void {
    this.dispatch(Event.NO_CREDS);
    this.render();
  }

  /** Start verification polling. */
  private startVerifyPolling(): void {
    this.stopVerifyPolling();

    // Mark all pending records as "pushing" to show activity
    for (const r of this.ctx.results) {
      if (r.status === "pending") {
        r.status = "pushing";
      }
    }

    this.verifyPoller = startVerifyPolling(this.ctx.options, this.ctx.results, {
      onRecordVerified: (_index: number) => {
        this.render();
      },
      onAllVerified: () => {
        this.dispatch(Event.ALL_VERIFIED);
        clearDCState(this.ctx.options.domain);
        this.ctx.options.onComplete?.(this.ctx.results);
        this.render();
      },
      onTimeout: () => {
        this.ctx.verifyTimedOut = true;
        this.stopVerifyPolling();
        this.render();
      },
    });
  }

  /** Stop verification polling. */
  private stopVerifyPolling(): void {
    if (this.verifyPoller) {
      this.verifyPoller.stop();
      this.verifyPoller = null;
    }
  }

  /** Manual re-verify after timeout. */
  private handleManualVerify(): void {
    this.ctx.verifyTimedOut = false;
    this.startVerifyPolling();
    this.render();
  }

  /** Re-render the modal content based on current state. */
  private render(): void {
    if (!this.overlay) {
      this.overlay = document.createElement("div");
      this.overlay.className = "de-overlay";
      this.overlay.setAttribute("role", "dialog");
      this.overlay.setAttribute("aria-modal", "true");
      this.overlay.setAttribute("aria-label", "DNS record setup");
      this.overlay.addEventListener("click", (e) => {
        if (e.target === this.overlay) this.close();
      });
      this.shadow.appendChild(this.overlay);
    }

    // Clear overlay children and rebuild via DOM API
    while (this.overlay.firstChild) {
      this.overlay.removeChild(this.overlay.firstChild);
    }

    this.overlay.appendChild(this.buildCard());
    // Don't steal focus during active verification polling
    if (!this.verifyPoller) {
      this.focusFirst();
    }
  }

  /** Build the full card element. */
  private buildCard(): HTMLElement {
    const card = el("div", { className: "de-card", role: "document" });
    card.appendChild(this.buildHeader());
    card.appendChild(this.buildBody());
    const footer = this.buildFooter();
    if (footer) card.appendChild(footer);
    return card;
  }

  /** Build the header with title and close button. */
  private buildHeader(): HTMLElement {
    const header = el("div", { className: "de-header" });

    const titleWrap = el("div");
    titleWrap.appendChild(el("h2", {}, ["Configure DNS"]));
    const sub = el("div", { className: "de-header-sub" });
    const domainSpan = el("span", { className: "de-domain" }, [this.ctx.options.domain]);
    sub.appendChild(domainSpan);
    titleWrap.appendChild(sub);
    header.appendChild(titleWrap);

    const closeBtn = el("button", { className: "de-close-btn", "aria-label": "Close dialog", type: "button" });
    setSvg(closeBtn, ICON_CLOSE);
    closeBtn.addEventListener("click", () => this.close());
    header.appendChild(closeBtn);

    return header;
  }

  /** Build body content based on current state. */
  private buildBody(): HTMLElement {
    const body = el("div", { className: "de-body" });

    switch (this.ctx.state) {
      case State.DETECTING:
        body.appendChild(buildDetecting());
        break;
      case State.DC_FLOW: {
        const dcEl = buildDCFlow(
          this.ctx.providerLabel,
          this.ctx.options.records.length,
          () => this.handleDCAutoSetup(),
          () => this.handleDCManual(),
        );
        dcEl.appendChild(this.buildManualFallbackLink());
        body.appendChild(dcEl);
        break;
      }
      case State.CREDENTIAL_FLOW: {
        const credEl = buildCredentialFlow(
          this.ctx.providerSlug,
          {
            onSubmit: (provider, creds) => this.handleCredentialSubmit(provider, creds),
            onNoCreds: () => this.handleCredentialNoCreds(),
          },
        );
        credEl.appendChild(this.buildManualFallbackLink());
        body.appendChild(credEl);
        break;
      }
      case State.FALLBACK:
        body.appendChild(this.buildFallback());
        break;
      case State.PUSHING:
        body.appendChild(
          buildPushProgress(
            this.ctx.results,
            this.ctx.pushErrorMessage || null,
            this.ctx.pushErrorMessage ? () => this.handlePushRetry() : null,
          ),
        );
        break;
      case State.VERIFYING:
        body.appendChild(
          buildVerifying(
            this.ctx.results,
            this.ctx.verifyTimedOut,
            () => this.handleManualVerify(),
          ),
        );
        break;
      case State.COMPLETE:
        body.appendChild(this.buildComplete());
        break;
      case State.ERROR:
        body.appendChild(this.buildError());
        break;
      default:
        body.appendChild(buildDetecting());
        break;
    }

    return body;
  }

  private buildFallback(): HTMLElement {
    const container = el("div", { className: "de-fallback" });

    // Provider-specific or generic instructions
    const provider = this.ctx.providerLabel;
    const intro = el("div", { className: "de-fallback-intro" });
    if (provider) {
      intro.appendChild(el("p", { className: "de-fallback-heading" },
        ["Log in to " + provider + " and add these DNS records:"]));
      const providerLink = PROVIDER_DNS_URLS[this.ctx.providerSlug || ""];
      if (providerLink) {
        const link = el("a", {
          className: "de-fallback-provider-link",
          href: providerLink,
          target: "_blank",
          rel: "noopener noreferrer",
        }, ["Open " + provider + " DNS settings"]);
        intro.appendChild(link);
      }
    } else {
      intro.appendChild(el("p", { className: "de-fallback-heading" },
        ["Log in to your DNS provider and add these records:"]));
    }
    container.appendChild(intro);

    // Record cards
    const list = el("div", { className: "de-records-list" });
    for (const r of this.ctx.results) {
      list.appendChild(this.buildRecordCard(r));
    }
    container.appendChild(list);

    return container;
  }

  /** Build a single record card for fallback view with type badge, name, value, and copy. */
  private buildRecordCard(result: RecordResult): HTMLElement {
    const r = result.record;
    const card = el("div", { className: "de-record-item" });

    // Header row: type badge + name
    const header = el("div", { className: "de-record-header" });
    header.appendChild(el("span", { className: "de-record-type" }, [r.type]));
    header.appendChild(el("span", { className: "de-record-name" }, [r.name]));
    card.appendChild(header);

    // Value row: code + copy button
    const valueRow = el("div", { className: "de-record-value" });
    valueRow.appendChild(el("code", {}, [r.content]));
    const copyBtn = el("button", {
      className: "de-copy-btn",
      "aria-label": "Copy value",
      type: "button",
    }) as HTMLButtonElement;
    setSvg(copyBtn, ICON_COPY);
    copyBtn.addEventListener("click", () => this.handleCopy(copyBtn, r.content));
    valueRow.appendChild(copyBtn);
    card.appendChild(valueRow);

    // TTL if provided
    if (r.ttl) {
      card.appendChild(el("div", { className: "de-record-ttl" }, ["TTL: " + r.ttl]));
    }

    return card;
  }

  /** Build the "Add records manually" link used in DC and credential screens. */
  private buildManualFallbackLink(): HTMLElement {
    const wrap = el("div", { className: "de-manual-link-wrap" });
    const link = el("button", {
      className: "de-btn de-btn-link de-manual-link",
      type: "button",
    }, ["Add records manually"]);
    link.addEventListener("click", () => {
      this.dispatch(Event.NO_CREDS);
      this.render();
    });
    wrap.appendChild(link);
    return wrap;
  }

  private buildComplete(): HTMLElement {
    const wrap = el("div", { className: "de-complete" });
    const circle = el("div", { className: "de-check-circle" });
    setSvg(circle, ICON_CHECK);
    wrap.appendChild(circle);
    wrap.appendChild(el("div", { className: "de-complete-title" }, ["DNS configured"]));

    const sub = el("div", { className: "de-complete-sub" });
    sub.appendChild(document.createTextNode("All records have been verified for "));
    sub.appendChild(el("span", { className: "de-domain" }, [this.ctx.options.domain]));
    wrap.appendChild(sub);

    return wrap;
  }

  private buildError(): HTMLElement {
    const wrap = el("div", { className: "de-error-view" });
    const icon = el("div", { className: "de-error-icon" });
    setSvg(icon, ICON_ALERT);
    wrap.appendChild(icon);
    wrap.appendChild(el("div", { className: "de-error-title" }, ["Something went wrong"]));
    wrap.appendChild(el("div", { className: "de-error-message" },
      [this.ctx.errorMessage || "An unexpected error occurred. Please try again."]));
    return wrap;
  }

  /** Build footer with action buttons based on state. */
  private buildFooter(): HTMLElement | null {
    const footer = el("div", { className: "de-footer" });

    switch (this.ctx.state) {
      case State.FALLBACK: {
        const cancelBtn = el("button", { className: "de-btn de-btn-secondary", type: "button" }, ["Cancel"]);
        cancelBtn.addEventListener("click", () => this.close());
        footer.appendChild(cancelBtn);

        const verifyBtn = el("button", { className: "de-btn de-btn-primary", type: "button" }, ["Verify records"]);
        verifyBtn.addEventListener("click", () => this.handleVerify());
        footer.appendChild(verifyBtn);
        return footer;
      }
      case State.PUSHING: {
        const cancelBtn = el("button", { className: "de-btn de-btn-secondary", type: "button" }, ["Cancel"]);
        cancelBtn.addEventListener("click", () => this.close());
        footer.appendChild(cancelBtn);
        if (!this.ctx.pushErrorMessage) {
          const pushingBtn = el("button", { className: "de-btn de-btn-primary", type: "button", disabled: "true" }, ["Pushing..."]);
          footer.appendChild(pushingBtn);
        }
        return footer;
      }
      case State.VERIFYING: {
        const cancelBtn = el("button", { className: "de-btn de-btn-secondary", type: "button" }, ["Cancel"]);
        cancelBtn.addEventListener("click", () => this.close());
        footer.appendChild(cancelBtn);

        if (!this.ctx.verifyTimedOut) {
          const verifyingBtn = el("button", { className: "de-btn de-btn-primary", type: "button", disabled: "true" }, ["Verifying..."]);
          footer.appendChild(verifyingBtn);
        }
        return footer;
      }
      case State.COMPLETE: {
        const doneBtn = el("button", { className: "de-btn de-btn-primary", type: "button" }, ["Done"]);
        doneBtn.addEventListener("click", () => this.close());
        footer.appendChild(doneBtn);
        return footer;
      }
      case State.ERROR: {
        const closeBtn = el("button", { className: "de-btn de-btn-secondary", type: "button" }, ["Close"]);
        closeBtn.addEventListener("click", () => this.close());
        footer.appendChild(closeBtn);

        const retryBtn = el("button", { className: "de-btn de-btn-primary", type: "button" }, ["Try again"]);
        retryBtn.addEventListener("click", () => {
          this.ctx.errorMessage = "";
          this.ctx.pushErrorMessage = "";
          this.ctx.verifyTimedOut = false;
          this.stopPushController();
          this.ctx.results = this.ctx.options.records.map((r) => ({
            record: r,
            status: "pending" as const,
          }));
          this.ctx.state = State.IDLE;
          this.dispatch(Event.OPEN);
          this.render();
          this.startDetection();
        });
        footer.appendChild(retryBtn);
        return footer;
      }
      default:
        return null;
    }
  }

  /** Copy record value to clipboard. */
  private async handleCopy(btn: HTMLButtonElement, value: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      const textarea = document.createElement("textarea");
      textarea.value = value;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
    }

    btn.classList.add("de-copied");
    while (btn.firstChild) btn.removeChild(btn.firstChild);
    setSvg(btn, ICON_CHECK);

    setTimeout(() => {
      btn.classList.remove("de-copied");
      while (btn.firstChild) btn.removeChild(btn.firstChild);
      setSvg(btn, ICON_COPY);
    }, 2000);
  }

  /** Handle credential form submission. */
  private handleCredentialSubmit(provider: string, creds: Record<string, string>): void {
    this.ctx.pushErrorMessage = "";
    this.ctx.providerSlug = provider;
    // Reset results to pending
    this.ctx.results = this.ctx.options.records.map((r) => ({
      record: r,
      status: "pending" as const,
    }));
    this.dispatch(Event.CREDS_SUBMITTED);

    this.stopPushController();
    this.pushController = startPushFlow(
      this.ctx.options,
      provider,
      creds,
      this.ctx.results,
      {
        onRender: () => this.render(),
        onAllDone: () => {
          const allVerified = this.ctx.results.every((r) => r.status === "verified");
          if (allVerified) {
            this.dispatch(Event.ALL_VERIFIED);
            this.ctx.options.onComplete?.(this.ctx.results);
          } else {
            // Partial failure: some records applied, some failed.
            // Transition to complete with partial results so user has a forward path.
            this.dispatch(Event.ALL_VERIFIED);
            this.ctx.options.onComplete?.(this.ctx.results);
          }
          this.render();
        },
        onError: (message) => {
          this.ctx.pushErrorMessage = message;
          // Reset results so retry shows clean state
          for (const r of this.ctx.results) {
            if (r.status === "pushing") {
              r.status = "pending";
            }
          }
          this.render();
        },
      },
    );

    this.render();
  }

  /** Handle "I don't have API access" from credential flow. */
  private handleCredentialNoCreds(): void {
    this.dispatch(Event.NO_CREDS);
    this.render();
  }

  /** Handle retry after push error. Goes back to credential form. */
  private handlePushRetry(): void {
    this.stopPushController();
    this.ctx.pushErrorMessage = "";
    this.ctx.results = this.ctx.options.records.map((r) => ({
      record: r,
      status: "pending" as const,
    }));
    // Go back to credential flow
    this.ctx.state = State.CREDENTIAL_FLOW;
    this.render();
  }

  /** Stop any active push controller. */
  private stopPushController(): void {
    if (this.pushController) {
      this.pushController.stop();
      this.pushController = null;
    }
  }

  /** Start real verification via API polling. */
  private handleVerify(): void {
    this.dispatch(Event.PUSH_STARTED);
    this.startVerifyPolling();
    this.render();
  }

  /** Focus first focusable element. */
  private focusFirst(): void {
    requestAnimationFrame(() => {
      const focusable = this.shadow.querySelector<HTMLElement>(
        'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
      );
      focusable?.focus();
    });
  }

  /** Keyboard event handler. */
  private handleKeydown = (e: Event): void => {
    const ke = e as KeyboardEvent;

    if (ke.key === "Escape") {
      ke.preventDefault();
      this.close();
      return;
    }

    if (ke.key === "Tab") {
      this.trapFocus(ke);
    }
  };

  /** Keep focus cycling within the modal. */
  private trapFocus(e: KeyboardEvent): void {
    const focusableEls = Array.from(
      this.shadow.querySelectorAll<HTMLElement>(
        'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
      )
    );

    if (focusableEls.length === 0) return;

    const first = focusableEls[0];
    const last = focusableEls[focusableEls.length - 1];
    const active = this.shadow.activeElement;

    if (e.shiftKey) {
      if (active === first || !active) {
        e.preventDefault();
        last.focus();
      }
    } else {
      if (active === last || !active) {
        e.preventDefault();
        first.focus();
      }
    }
  }
}
