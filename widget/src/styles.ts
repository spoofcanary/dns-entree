/** All widget CSS. Injected into shadow DOM as a <style> block. */
export const styles = `
  *,
  *::before,
  *::after {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
  }

  :host {
    all: initial;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    font-size: 14px;
    line-height: 1.5;
    color: var(--de-fg);
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  /* --- Overlay --- */
  .de-overlay {
    position: fixed;
    inset: 0;
    z-index: 2147483647;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--de-overlay);
    opacity: 0;
    transition: opacity 0.2s ease;
  }

  .de-overlay.de-visible {
    opacity: 1;
  }

  /* --- Card --- */
  .de-card {
    position: relative;
    width: 100%;
    max-width: 480px;
    max-height: 90vh;
    overflow-y: auto;
    background: var(--de-bg);
    border-radius: var(--de-radius);
    box-shadow: var(--de-shadow);
    transform: translateY(16px) scale(0.98);
    transition: transform 0.25s cubic-bezier(0.16, 1, 0.3, 1), opacity 0.2s ease;
    opacity: 0;
  }

  .de-overlay.de-visible .de-card {
    transform: translateY(0) scale(1);
    opacity: 1;
  }

  /* Mobile: fill screen */
  @media (max-width: 480px) {
    .de-card {
      max-width: 100%;
      max-height: 100vh;
      height: 100%;
      border-radius: 0;
    }
  }

  /* --- Header --- */
  .de-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--de-border);
  }

  .de-header h2 {
    font-size: 16px;
    font-weight: 600;
    color: var(--de-fg);
    margin: 0;
  }

  .de-header-sub {
    font-size: 12px;
    color: var(--de-fg-muted);
    font-weight: 400;
    margin-top: 2px;
  }

  .de-close-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border: none;
    background: transparent;
    color: var(--de-fg-muted);
    cursor: pointer;
    border-radius: 4px;
    transition: background 0.15s ease, color 0.15s ease;
    flex-shrink: 0;
  }

  .de-close-btn:hover {
    background: var(--de-bg-secondary);
    color: var(--de-fg);
  }

  .de-close-btn:focus-visible {
    outline: 2px solid var(--de-accent);
    outline-offset: 2px;
  }

  .de-close-btn svg {
    width: 16px;
    height: 16px;
  }

  /* --- Body --- */
  .de-body {
    padding: 20px;
  }

  /* --- State: Detecting (spinner) --- */
  .de-detecting {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    padding: 32px 0;
  }

  .de-spinner {
    width: 32px;
    height: 32px;
    border: 3px solid var(--de-border);
    border-top-color: var(--de-accent);
    border-radius: 50%;
    animation: de-spin 0.7s linear infinite;
  }

  @keyframes de-spin {
    to { transform: rotate(360deg); }
  }

  .de-detecting-text {
    color: var(--de-fg-muted);
    font-size: 13px;
  }

  /* --- State: Fallback (copy-paste records) --- */
  .de-records-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .de-record-item {
    background: var(--de-bg-secondary);
    border: 1px solid var(--de-border);
    border-radius: calc(var(--de-radius) - 2px);
    padding: 12px;
  }

  .de-record-header {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }

  .de-record-type {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding: 2px 6px;
    border-radius: 4px;
    background: var(--de-accent);
    color: var(--de-accent-fg);
  }

  .de-record-name {
    font-size: 13px;
    color: var(--de-fg-muted);
    font-family: "SF Mono", "Fira Code", "Fira Mono", Menlo, Consolas, monospace;
  }

  .de-record-value {
    display: flex;
    align-items: stretch;
    gap: 0;
    margin-top: 4px;
  }

  .de-record-value code {
    flex: 1;
    display: block;
    padding: 8px 10px;
    font-size: 12px;
    font-family: "SF Mono", "Fira Code", "Fira Mono", Menlo, Consolas, monospace;
    background: var(--de-bg);
    border: 1px solid var(--de-border);
    border-right: none;
    border-radius: calc(var(--de-radius) - 2px) 0 0 calc(var(--de-radius) - 2px);
    color: var(--de-fg);
    word-break: break-all;
    white-space: pre-wrap;
    line-height: 1.4;
  }

  .de-copy-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    flex-shrink: 0;
    border: 1px solid var(--de-border);
    border-left: none;
    border-radius: 0 calc(var(--de-radius) - 2px) calc(var(--de-radius) - 2px) 0;
    background: var(--de-bg);
    color: var(--de-fg-muted);
    cursor: pointer;
    transition: background 0.15s ease, color 0.15s ease;
  }

  .de-copy-btn:hover {
    background: var(--de-bg-secondary);
    color: var(--de-accent);
  }

  .de-copy-btn:focus-visible {
    outline: 2px solid var(--de-accent);
    outline-offset: -2px;
  }

  .de-copy-btn svg {
    width: 14px;
    height: 14px;
  }

  .de-copy-btn.de-copied {
    color: var(--de-success);
  }

  /* --- Status indicators --- */
  .de-record-status {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 8px;
    font-size: 12px;
  }

  .de-status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .de-status-dot.de-pending { background: var(--de-fg-muted); }
  .de-status-dot.de-pushing { background: var(--de-accent); animation: de-pulse 1s ease infinite; }
  .de-status-dot.de-verified { background: var(--de-success); }
  .de-status-dot.de-failed { background: var(--de-error); }

  @keyframes de-pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }

  .de-status-text {
    color: var(--de-fg-muted);
  }

  .de-status-text.de-verified { color: var(--de-success); }
  .de-status-text.de-failed { color: var(--de-error); }

  /* --- State: Complete --- */
  .de-complete {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 24px 0;
    text-align: center;
  }

  .de-check-circle {
    width: 48px;
    height: 48px;
    border-radius: 50%;
    background: var(--de-success);
    display: flex;
    align-items: center;
    justify-content: center;
    animation: de-pop 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  @keyframes de-pop {
    0% { transform: scale(0.5); opacity: 0; }
    100% { transform: scale(1); opacity: 1; }
  }

  .de-check-circle svg {
    width: 24px;
    height: 24px;
    color: #ffffff;
  }

  .de-complete-title {
    font-size: 16px;
    font-weight: 600;
    color: var(--de-fg);
  }

  .de-complete-sub {
    font-size: 13px;
    color: var(--de-fg-muted);
  }

  /* --- State: Error --- */
  .de-error-view {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 24px 0;
    text-align: center;
  }

  .de-error-icon {
    width: 48px;
    height: 48px;
    border-radius: 50%;
    background: var(--de-error);
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .de-error-icon svg {
    width: 24px;
    height: 24px;
    color: #ffffff;
  }

  .de-error-title {
    font-size: 16px;
    font-weight: 600;
    color: var(--de-fg);
  }

  .de-error-message {
    font-size: 13px;
    color: var(--de-fg-muted);
    max-width: 320px;
  }

  /* --- Buttons --- */
  .de-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 8px 16px;
    font-size: 13px;
    font-weight: 500;
    font-family: inherit;
    border-radius: calc(var(--de-radius) - 2px);
    cursor: pointer;
    transition: background 0.15s ease, transform 0.1s ease;
    border: none;
  }

  .de-btn:active {
    transform: scale(0.97);
  }

  .de-btn:focus-visible {
    outline: 2px solid var(--de-accent);
    outline-offset: 2px;
  }

  .de-btn-primary {
    background: var(--de-accent);
    color: var(--de-accent-fg);
  }

  .de-btn-primary:hover {
    background: var(--de-accent-hover);
  }

  .de-btn-secondary {
    background: var(--de-bg-secondary);
    color: var(--de-fg);
    border: 1px solid var(--de-border);
  }

  .de-btn-secondary:hover {
    background: var(--de-border);
  }

  /* --- Footer --- */
  .de-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    padding: 12px 20px;
    border-top: 1px solid var(--de-border);
  }

  /* --- Misc --- */
  .de-domain {
    font-family: "SF Mono", "Fira Code", "Fira Mono", Menlo, Consolas, monospace;
    font-size: 13px;
    color: var(--de-accent);
  }

  .de-sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border-width: 0;
  }

  /* --- State: DC Flow --- */
  .de-dc-flow {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .de-dc-badge {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 14px;
    background: var(--de-bg-secondary);
    border: 1px solid var(--de-border);
    border-radius: calc(var(--de-radius) - 2px);
    font-size: 14px;
    font-weight: 500;
    color: var(--de-fg);
  }

  .de-dc-bolt {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    flex-shrink: 0;
    color: var(--de-accent);
  }

  .de-dc-bolt svg {
    width: 16px;
    height: 16px;
  }

  .de-dc-desc {
    font-size: 13px;
    color: var(--de-fg-muted);
    line-height: 1.5;
  }

  .de-dc-summary {
    padding: 10px 14px;
    background: var(--de-bg-secondary);
    border: 1px solid var(--de-border);
    border-radius: calc(var(--de-radius) - 2px);
  }

  .de-dc-summary-label {
    font-size: 13px;
    color: var(--de-fg-muted);
    font-weight: 500;
  }

  .de-dc-buttons {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding-top: 4px;
  }

  .de-dc-auto-btn {
    width: 100%;
    padding: 10px 16px;
    font-size: 14px;
  }

  .de-dc-btn-bolt {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
  }

  .de-dc-btn-bolt svg {
    width: 14px;
    height: 14px;
  }

  .de-btn-link {
    background: none;
    border: none;
    color: var(--de-fg-muted);
    font-size: 12px;
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 4px;
    transition: color 0.15s ease;
  }

  .de-btn-link:hover {
    color: var(--de-fg);
  }

  .de-btn-link:focus-visible {
    outline: 2px solid var(--de-accent);
    outline-offset: 2px;
  }

  /* --- State: Verifying (polling) --- */
  .de-verify-item {
    padding: 10px 12px;
  }

  .de-verify-status {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 8px;
    font-size: 12px;
  }

  .de-verify-check {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: var(--de-success);
    flex-shrink: 0;
    animation: de-pop 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .de-verify-check svg {
    width: 12px;
    height: 12px;
    color: #ffffff;
  }

  .de-verify-fail {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: var(--de-error);
    flex-shrink: 0;
    animation: de-pop 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .de-verify-fail svg {
    width: 12px;
    height: 12px;
    color: #ffffff;
  }

  /* --- State: Credential Flow --- */
  .de-cred-flow {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .de-cred-provider {
    font-size: 14px;
    font-weight: 600;
    color: var(--de-fg);
  }

  .de-cred-fields {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .de-cred-field-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .de-cred-label {
    font-size: 12px;
    font-weight: 500;
    color: var(--de-fg-muted);
  }

  .de-cred-input-wrap {
    display: flex;
    align-items: stretch;
    border: 1px solid var(--de-border);
    border-radius: calc(var(--de-radius) - 2px);
    background: var(--de-bg);
    transition: border-color 0.15s ease;
  }

  .de-cred-input-wrap:focus-within {
    border-color: var(--de-accent);
  }

  .de-cred-input {
    flex: 1;
    padding: 8px 10px;
    font-size: 13px;
    font-family: "SF Mono", "Fira Code", "Fira Mono", Menlo, Consolas, monospace;
    background: transparent;
    border: none;
    color: var(--de-fg);
    outline: none;
    min-width: 0;
  }

  .de-cred-input::placeholder {
    color: var(--de-fg-muted);
    opacity: 0.6;
  }

  .de-cred-toggle {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    flex-shrink: 0;
    background: transparent;
    border: none;
    border-left: 1px solid var(--de-border);
    color: var(--de-fg-muted);
    cursor: pointer;
    transition: color 0.15s ease;
  }

  .de-cred-toggle:hover {
    color: var(--de-fg);
  }

  .de-cred-toggle svg {
    width: 14px;
    height: 14px;
  }

  .de-cred-security {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 10px 12px;
    background: var(--de-bg-secondary);
    border: 1px solid var(--de-border);
    border-radius: calc(var(--de-radius) - 2px);
    font-size: 12px;
    color: var(--de-fg-muted);
    line-height: 1.4;
  }

  .de-cred-lock-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
    flex-shrink: 0;
    margin-top: 1px;
  }

  .de-cred-lock-icon svg {
    width: 14px;
    height: 14px;
  }

  .de-cred-help {
    border-top: 1px solid var(--de-border);
    padding-top: 12px;
  }

  .de-cred-help-toggle {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 0;
    background: none;
    border: none;
    font-size: 12px;
    font-family: inherit;
    color: var(--de-fg-muted);
    cursor: pointer;
    transition: color 0.15s ease;
    text-align: left;
  }

  .de-cred-help-toggle:hover {
    color: var(--de-fg);
  }

  .de-cred-help-toggle:focus-visible {
    outline: 2px solid var(--de-accent);
    outline-offset: 2px;
    border-radius: 2px;
  }

  .de-cred-help-chevron {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 14px;
    height: 14px;
    transition: transform 0.2s ease;
  }

  .de-cred-help-chevron svg {
    width: 12px;
    height: 12px;
  }

  .de-cred-help-chevron.de-expanded {
    transform: rotate(180deg);
  }

  .de-cred-help-body {
    padding: 10px 0 0 0;
  }

  .de-cred-help-body p {
    font-size: 12px;
    color: var(--de-fg-muted);
    line-height: 1.5;
    margin: 0 0 8px 0;
  }

  .de-cred-help-link {
    font-size: 12px;
    color: var(--de-accent);
    text-decoration: none;
    transition: opacity 0.15s ease;
  }

  .de-cred-help-link:hover {
    opacity: 0.8;
  }

  .de-cred-actions {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding-top: 4px;
  }

  .de-cred-submit {
    width: 100%;
    padding: 10px 16px;
    font-size: 14px;
  }

  .de-cred-submit:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .de-btn[disabled] {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* --- State: Fallback (enhanced) --- */
  .de-fallback-intro {
    margin-bottom: 16px;
  }

  .de-fallback-heading {
    font-size: 13px;
    color: var(--de-fg-muted);
    margin: 0 0 6px 0;
  }

  .de-fallback-provider-link {
    display: inline-block;
    font-size: 12px;
    color: var(--de-accent);
    text-decoration: none;
    transition: opacity 0.15s ease;
  }

  .de-fallback-provider-link:hover {
    opacity: 0.8;
  }

  .de-record-ttl {
    font-size: 11px;
    color: var(--de-fg-muted);
    margin-top: 6px;
  }

  .de-manual-link-wrap {
    border-top: 1px solid var(--de-border);
    margin-top: 16px;
    padding-top: 12px;
    text-align: center;
  }

  .de-manual-link {
    font-size: 12px;
  }
`;
