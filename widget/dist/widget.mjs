var _=`
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
`;var d={IDLE:"idle",DETECTING:"detecting",DC_FLOW:"dc_flow",CREDENTIAL_FLOW:"credential_flow",FALLBACK:"fallback",PUSHING:"pushing",VERIFYING:"verifying",COMPLETE:"complete",ERROR:"error"},c={OPEN:"OPEN",DETECTED:"DETECTED",DC_AVAILABLE:"DC_AVAILABLE",DC_RETURN:"DC_RETURN",CREDS_SUBMITTED:"CREDS_SUBMITTED",NO_CREDS:"NO_CREDS",PUSH_STARTED:"PUSH_STARTED",RECORD_VERIFIED:"RECORD_VERIFIED",ALL_VERIFIED:"ALL_VERIFIED",ERROR:"ERROR",CLOSE:"CLOSE"},he={[d.IDLE]:{[c.OPEN]:d.DETECTING,[c.DC_RETURN]:d.VERIFYING},[d.DETECTING]:{[c.DC_AVAILABLE]:d.DC_FLOW,[c.DETECTED]:d.CREDENTIAL_FLOW,[c.NO_CREDS]:d.FALLBACK,[c.ERROR]:d.ERROR,[c.CLOSE]:d.IDLE},[d.DC_FLOW]:{[c.PUSH_STARTED]:d.PUSHING,[c.DC_RETURN]:d.VERIFYING,[c.NO_CREDS]:d.FALLBACK,[c.ERROR]:d.ERROR,[c.CLOSE]:d.IDLE},[d.CREDENTIAL_FLOW]:{[c.CREDS_SUBMITTED]:d.PUSHING,[c.NO_CREDS]:d.FALLBACK,[c.ERROR]:d.ERROR,[c.CLOSE]:d.IDLE},[d.FALLBACK]:{[c.PUSH_STARTED]:d.VERIFYING,[c.ERROR]:d.ERROR,[c.CLOSE]:d.IDLE},[d.PUSHING]:{[c.RECORD_VERIFIED]:d.VERIFYING,[c.ALL_VERIFIED]:d.COMPLETE,[c.ERROR]:d.ERROR,[c.CLOSE]:d.IDLE},[d.VERIFYING]:{[c.ALL_VERIFIED]:d.COMPLETE,[c.ERROR]:d.ERROR,[c.CLOSE]:d.IDLE},[d.COMPLETE]:{[c.CLOSE]:d.IDLE},[d.ERROR]:{[c.CLOSE]:d.IDLE,[c.OPEN]:d.DETECTING}};function B(n,e){return he[n]?.[e]??n}var me={"--de-bg":"#ffffff","--de-bg-secondary":"#f7f8fa","--de-fg":"#1a1a2e","--de-fg-muted":"#6b7280","--de-accent":"#2563eb","--de-accent-hover":"#1d4ed8","--de-accent-fg":"#ffffff","--de-border":"#e5e7eb","--de-success":"#16a34a","--de-error":"#dc2626","--de-overlay":"rgba(0, 0, 0, 0.5)","--de-radius":"8px","--de-shadow":"0 20px 60px rgba(0, 0, 0, 0.15), 0 4px 16px rgba(0, 0, 0, 0.08)"},fe={"--de-bg":"#1a1a2e","--de-bg-secondary":"#232340","--de-fg":"#e5e7eb","--de-fg-muted":"#9ca3af","--de-accent":"#3b82f6","--de-accent-hover":"#60a5fa","--de-accent-fg":"#ffffff","--de-border":"#374151","--de-success":"#22c55e","--de-error":"#ef4444","--de-overlay":"rgba(0, 0, 0, 0.7)","--de-radius":"8px","--de-shadow":"0 20px 60px rgba(0, 0, 0, 0.4), 0 4px 16px rgba(0, 0, 0, 0.25)"};function ge(){return typeof window>"u"?"light":window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light"}function F(n){return n==="light"||n==="dark"?n:ge()}function M(n,e){let t=n==="dark"?{...fe}:{...me};return e&&(t["--de-accent"]=e),t}function V(n,e){for(let[t,r]of Object.entries(e))n.style.setProperty(t,r)}async function A(n,e,t,r){let o={"Content-Type":"application/json"};n.apiKey&&(o.Authorization="Bearer "+n.apiKey);let i;try{i=await fetch(n.apiUrl.replace(/\/+$/,"")+t,{method:e,headers:o,body:r!==void 0?JSON.stringify(r):void 0,credentials:"omit"})}catch(h){return{ok:!1,error:h instanceof Error?h.message:"Network error"}}let s;try{s=await i.json()}catch{return{ok:!1,error:"Invalid JSON response"}}let a=s;if(a.ok===!0&&a.data!==void 0)return{ok:!0,data:a.data};let l=a.error;return l?{ok:!1,error:l.message||"Unknown error",code:l.code,details:l.details}:{ok:!1,error:"Unexpected response format"}}function z(n,e){return A(n,"POST","/v1/detect",{domain:e})}function j(n,e){return A(n,"POST","/v1/dc/discover",{domain:e})}function K(n,e){return A(n,"POST","/v1/dc/apply-url",e)}function O(n,e,t,r,o){return A(n,"POST","/v1/verify",{domain:e,type:t,name:r,contains:o})}function G(n,e,t,r,o){let i={"Content-Type":"application/json","X-Entree-Provider":e};n.apiKey&&(i.Authorization="Bearer "+n.apiKey);for(let[s,a]of Object.entries(t))i[s]=a;return fetch(n.apiUrl.replace(/\/+$/,"")+"/v1/apply",{method:"POST",headers:i,body:JSON.stringify({domain:r,records:o}),credentials:"omit"}).then(s=>s.json()).then(s=>{let a=s;if(a.ok===!0&&a.data!==void 0)return{ok:!0,data:a.data};let l=a.error;return l?{ok:!1,error:l.message||"Unknown error",code:l.code,details:l.details}:{ok:!1,error:"Unexpected response format"}}).catch(s=>({ok:!1,error:s instanceof Error?s.message:"Network error"}))}function U(){let n=document.createElement("div");n.className="de-detecting";let e=document.createElement("div");e.className="de-spinner",n.appendChild(e);let t=document.createElement("div");return t.className="de-detecting-text",t.textContent="Checking your DNS provider...",n.appendChild(t),n}async function W(n){let e={apiUrl:n.apiUrl,apiKey:n.apiKey},[t,r]=await Promise.all([z(e,n.domain),j(e,n.domain)]);if(r.ok&&r.data.Supported){let o=r.data.ProviderName||(t.ok?t.data.label:"");return{type:"dc",provider:r.data.ProviderID,label:r.data.ProviderName||o,dcDiscovery:r.data,detection:t.ok?t.data:void 0}}return t.ok&&t.data.supported&&t.data.provider?{type:"credential",provider:t.data.provider,label:t.data.label,detection:t.data}:t.ok&&t.data.provider?{type:"fallback",provider:t.data.provider,label:t.data.label,detection:t.data}:{type:"fallback"}}var Y='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>',I="de_state_";function q(n,e,t){let r={domain:n.domain,records:n.records,dcDiscovery:e,providerLabel:t,timestamp:Date.now()};try{sessionStorage.setItem(I+n.domain,JSON.stringify(r))}catch{}}function J(n){try{let e=sessionStorage.getItem(I+n);if(!e)return null;let t=JSON.parse(e);return Date.now()-t.timestamp>600*1e3?(sessionStorage.removeItem(I+n),null):t}catch{return null}}function Z(n){try{sessionStorage.removeItem(I+n)}catch{}}function X(n,e){let t=document.createElement("template");t.innerHTML=e,n.appendChild(t.content)}function $(n,e,t,r){let o=document.createElement("div");o.className="de-dc-flow";let i=document.createElement("div");i.className="de-dc-badge";let s=document.createElement("span");s.className="de-dc-bolt",X(s,Y),i.appendChild(s);let a=document.createElement("span");a.textContent=n+" supports one-click setup",i.appendChild(a),o.appendChild(i);let l=document.createElement("p");l.className="de-dc-desc",l.textContent=e===1?"We can configure your DNS record automatically. You will be redirected to "+n+" to approve the changes.":"We can configure all "+e+" DNS records automatically. You will be redirected to "+n+" to approve the changes.",o.appendChild(l);let h=document.createElement("div");h.className="de-dc-summary";let g=document.createElement("div");g.className="de-dc-summary-label",g.textContent=e+" record"+(e!==1?"s":"")+" will be configured",h.appendChild(g),o.appendChild(h);let u=document.createElement("div");u.className="de-dc-buttons";let m=document.createElement("button");m.className="de-btn de-btn-primary de-dc-auto-btn",m.type="button";let f=document.createElement("span");f.className="de-dc-btn-bolt",X(f,Y),m.appendChild(f),m.appendChild(document.createTextNode("Set up automatically")),m.addEventListener("click",t),u.appendChild(m);let E=document.createElement("button");return E.className="de-btn de-btn-link",E.type="button",E.textContent="Set up manually instead",E.addEventListener("click",r),u.appendChild(E),o.appendChild(u),o}function Q(n,e){let t=e||window.location.href,r=new URL(t);return r.searchParams.delete("de_state"),r.searchParams.set("de_state",n),r.toString()}function ee(){try{return new URLSearchParams(window.location.search).get("de_state")}catch{return null}}function H(){try{let n=new URL(window.location.href);n.searchParams.has("de_state")&&(n.searchParams.delete("de_state"),window.history.replaceState({},"",n.toString()))}catch{}}var ve=3e3,ye=120*1e3,xe='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>';function be(n,e){let t=document.createElement("template");t.innerHTML=e,n.appendChild(t.content)}function te(n,e,t){let r={apiUrl:n.apiUrl,apiKey:n.apiKey},o=!1,i=null,s=Date.now();async function a(){if(o)return;let l=e.map((g,u)=>({r:g,i:u})).filter(g=>g.r.status!=="verified");if(l.length===0){t.onAllVerified();return}if(Date.now()-s>ye){t.onTimeout();return}let h=l.map(async({r:g,i:u})=>{let m=await O(r,n.domain,g.record.type,g.record.name,g.record.content);m.ok&&m.data.verified&&(e[u].status="verified",t.onRecordVerified(u))});if(await Promise.all(h),e.every(g=>g.status==="verified")){t.onAllVerified();return}o||(i=setTimeout(a,ve))}return a(),{stop(){o=!0,i!==null&&(clearTimeout(i),i=null)}}}function re(n,e,t){let r=document.createElement("div"),o=document.createElement("p");o.style.cssText="margin-bottom: 16px; color: var(--de-fg-muted); font-size: 13px;",e?o.textContent="DNS changes can take a few minutes to propagate.":o.textContent="Checking DNS propagation...",r.appendChild(o);let i=document.createElement("div");i.className="de-records-list";for(let s of n)i.appendChild(Ce(s));if(r.appendChild(i),e&&t){let s=document.createElement("div");s.style.cssText="margin-top: 16px; text-align: center;";let a=document.createElement("button");a.className="de-btn de-btn-primary",a.type="button",a.textContent="Check again",a.addEventListener("click",t),s.appendChild(a),r.appendChild(s)}return r}function Ce(n){let e=n.record,t=document.createElement("div");t.className="de-record-item de-verify-item";let r=document.createElement("div");r.className="de-record-header";let o=document.createElement("span");o.className="de-record-type",o.textContent=e.type,r.appendChild(o);let i=document.createElement("span");i.className="de-record-name",i.textContent=e.name,r.appendChild(i),t.appendChild(r);let s=document.createElement("div");if(s.className="de-verify-status",n.status==="verified"){let a=document.createElement("span");a.className="de-verify-check",be(a,xe),s.appendChild(a);let l=document.createElement("span");l.className="de-status-text de-verified",l.textContent="Verified",s.appendChild(l)}else{let a=document.createElement("span");a.className="de-status-dot de-pushing",s.appendChild(a);let l=document.createElement("span");l.className="de-status-text",l.textContent="Waiting for propagation...",s.appendChild(l)}return t.appendChild(s),t}var Ee={cloudflare:{slug:"cloudflare",label:"Cloudflare",fields:[{key:"api_token",label:"API Token",placeholder:"Paste your Cloudflare API token",secret:!0,header:"X-Entree-Cloudflare-Token"}],helpUrl:"https://developers.cloudflare.com/fundamentals/api/get-started/create-token/",helpText:"Create a Custom Token with Zone > DNS > Edit permission. Select the specific zone (domain) you want to manage."},route53:{slug:"route53",label:"Amazon Route 53",fields:[{key:"access_key_id",label:"Access Key ID",placeholder:"AKIA...",secret:!1,header:"X-Entree-AWS-Access-Key-Id"},{key:"secret_access_key",label:"Secret Access Key",placeholder:"Paste your secret access key",secret:!0,header:"X-Entree-AWS-Secret-Access-Key"}],helpUrl:"https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_access-keys.html",helpText:"Create an IAM user with AmazonRoute53FullAccess policy, then generate an access key pair in the Security Credentials tab."},godaddy:{slug:"godaddy",label:"GoDaddy",fields:[{key:"api_key",label:"API Key",placeholder:"Paste your API key",secret:!1,header:"X-Entree-GoDaddy-Key"},{key:"api_secret",label:"API Secret",placeholder:"Paste your API secret",secret:!0,header:"X-Entree-GoDaddy-Secret"}],helpUrl:"https://developer.godaddy.com/keys",helpText:"Go to the GoDaddy Developer Portal and create a Production API key. Note: API access requires a paid plan or Discount Domain Club membership."},google_cloud_dns:{slug:"google_cloud_dns",label:"Google Cloud DNS",fields:[{key:"service_account_json",label:"Service Account JSON",placeholder:"Paste the full JSON key file contents",secret:!0,header:"X-Entree-GCDNS-Service-Account-JSON"},{key:"project_id",label:"Project ID",placeholder:"my-project-123",secret:!1,header:"X-Entree-GCDNS-Project-Id"}],helpUrl:"https://cloud.google.com/iam/docs/keys-create-delete",helpText:"Create a service account with DNS Administrator role, then create and download a JSON key for that account."}},ke={slug:"generic",label:"DNS Provider",fields:[{key:"api_token",label:"Provider API Token",placeholder:"Paste your API token",secret:!0,header:"X-Entree-Cloudflare-Token"}],helpUrl:"",helpText:"Enter the API token for your DNS provider."};function we(n){return Ee[n]||{...ke,label:n||"DNS Provider"}}var De='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>',ne='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>',Re='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>',Ne='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>',Se='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>',Le='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>';function N(n,e){let t=document.createElement("template");t.innerHTML=e,n.appendChild(t.content)}function oe(n,e){let t=we(n),r=document.createElement("div");r.className="de-cred-flow";let o=document.createElement("div");o.className="de-cred-provider",o.textContent=t.label,r.appendChild(o);let i={},s=[],a=document.createElement("div");a.className="de-cred-fields";for(let y of t.fields){i[y.key]="";let x=document.createElement("div");x.className="de-cred-field-group";let C=document.createElement("label");C.className="de-cred-label",C.textContent=y.label;let k="de-cred-"+y.key;C.setAttribute("for",k),x.appendChild(C);let R=document.createElement("div");R.className="de-cred-input-wrap";let b=document.createElement("input");if(b.type=y.secret?"password":"text",b.id=k,b.className="de-cred-input",b.placeholder=y.placeholder,b.autocomplete="off",b.spellcheck=!1,b.setAttribute("data-field-key",y.key),b.addEventListener("input",()=>{i[y.key]=b.value,E()}),s.push(b),R.appendChild(b),y.secret){let v=document.createElement("button");v.type="button",v.className="de-cred-toggle",v.setAttribute("aria-label","Show password"),v.tabIndex=-1,N(v,ne);let L=!1;v.addEventListener("click",()=>{for(L=!L,b.type=L?"text":"password",v.setAttribute("aria-label",L?"Hide password":"Show password");v.firstChild;)v.removeChild(v.firstChild);N(v,L?Re:ne)}),R.appendChild(v)}x.appendChild(R),a.appendChild(x)}r.appendChild(a);let l=document.createElement("div");l.className="de-cred-security";let h=document.createElement("span");h.className="de-cred-lock-icon",N(h,De),l.appendChild(h);let g=document.createElement("span");if(g.textContent="Credentials are sent directly to your DNS provider's API. They are not stored.",l.appendChild(g),r.appendChild(l),t.helpText){let y=document.createElement("div");y.className="de-cred-help";let x=document.createElement("button");x.type="button",x.className="de-cred-help-toggle",x.textContent="How to get these credentials";let C=document.createElement("span");C.className="de-cred-help-chevron",N(C,Ne),x.appendChild(C);let k=document.createElement("div");k.className="de-cred-help-body",k.style.display="none";let R=document.createElement("p");if(R.textContent=t.helpText,k.appendChild(R),t.helpUrl){let v=document.createElement("a");v.href=t.helpUrl,v.target="_blank",v.rel="noopener noreferrer",v.className="de-cred-help-link",v.textContent="View documentation",k.appendChild(v)}let b=!1;x.addEventListener("click",()=>{b=!b,k.style.display=b?"block":"none",C.classList.toggle("de-expanded",b),x.setAttribute("aria-expanded",String(b))}),y.appendChild(x),y.appendChild(k),r.appendChild(y)}let u=document.createElement("div");u.className="de-cred-actions";let m=document.createElement("button");m.type="button",m.className="de-btn de-btn-primary de-cred-submit",m.textContent="Push records",m.disabled=!0,m.addEventListener("click",()=>{let y={};for(let x of t.fields){let C=i[x.key];if(x.header==="X-Entree-GCDNS-Service-Account-JSON"&&C)try{C=btoa(C)}catch{C=btoa(unescape(encodeURIComponent(C)))}y[x.header]=C}e.onSubmit(t.slug==="generic"?n:t.slug,y)}),u.appendChild(m);let f=document.createElement("button");f.type="button",f.className="de-btn de-btn-link",f.textContent="I don't have API access",f.addEventListener("click",e.onNoCreds),u.appendChild(f),r.appendChild(u);function E(){let y=t.fields.every(x=>i[x.key].trim().length>0);m.disabled=!y}return r}var Te=3e3,Ae=120*1e3;function ie(n,e,t,r,o){let i={apiUrl:n.apiUrl,apiKey:n.apiKey},s=!1,a=null;for(let h of r)h.status="pushing";o.onRender(),G(i,e,t,n.domain,n.records).then(h=>{if(s)return;if(!h.ok){let u=Oe(h.error,h.code,e);o.onError(u);return}let g=h.data;for(let u=0;u<r.length;u++){let m=g.results.find(f=>f.type===r[u].record.type&&f.name===r[u].record.name);m&&(m.status==="error"||m.status==="failed"?(r[u].status="failed",r[u].error=m.verify_error||"Failed to apply record"):m.verified?r[u].status="verified":r[u].status="pushing")}for(let u of Object.keys(t))t[u]="";if(o.onRender(),r.every(u=>u.status==="verified"||u.status==="failed")){r.some(u=>u.status==="verified")&&o.onAllDone();return}l()}).catch(h=>{s||o.onError(h instanceof Error?h.message:"Cannot reach DNS service")});function l(){let h=Date.now();async function g(){if(s)return;if(Date.now()-h>Ae){o.onRender(),o.onAllDone();return}let u=r.filter(f=>f.status==="pushing");if(u.length===0){o.onAllDone();return}let m=u.map(async f=>{let E=await O(i,n.domain,f.record.type,f.record.name,f.record.content);E.ok&&E.data.verified&&(f.status="verified",o.onRender())});if(await Promise.all(m),r.every(f=>f.status==="verified"||f.status==="failed")){o.onAllDone();return}s||(a=setTimeout(g,Te))}g()}return{stop(){s=!0,a!==null&&(clearTimeout(a),a=null)}}}function Oe(n,e,t){if(e==="missing_credentials"||e==="invalid_credentials")switch(t){case"cloudflare":return"Invalid API token. Check that your token has Zone > DNS > Edit permission.";case"route53":return"Invalid AWS credentials. Verify your Access Key ID and Secret Access Key.";case"godaddy":return"Invalid GoDaddy API credentials. Check your key and secret, and ensure API access is enabled.";case"google_cloud_dns":return"Invalid service account JSON. Ensure the account has DNS Administrator role.";default:return"Invalid credentials. Check your API token and try again."}return e==="rate_limited"?"Rate limit reached. Please wait a moment and try again.":e==="zone_not_found"?"Domain zone not found. Check that this domain exists in your "+t+" account.":n.includes("Network error")||n.includes("fetch")?"Cannot reach DNS service. Check your connection and try again.":n||"Failed to apply records. Please try again."}function se(n,e,t){let r=document.createElement("div"),o=document.createElement("p");if(o.style.cssText="margin-bottom: 16px; color: var(--de-fg-muted); font-size: 13px;",e)o.textContent=e,o.style.color="var(--de-error)";else{let s=n.every(l=>l.status==="verified"),a=n.some(l=>l.status==="failed");if(s)o.textContent="All records applied and verified.";else if(a){let l=n.filter(h=>h.status==="verified").length;o.textContent=l+" of "+n.length+" records applied."}else o.textContent="Pushing records to your DNS provider..."}r.appendChild(o);let i=document.createElement("div");i.className="de-records-list";for(let s of n)i.appendChild(Ie(s));if(r.appendChild(i),e&&t){let s=document.createElement("div");s.style.cssText="margin-top: 16px; text-align: center;";let a=document.createElement("button");a.className="de-btn de-btn-primary",a.type="button",a.textContent="Try again",a.addEventListener("click",t),s.appendChild(a),r.appendChild(s)}return r}function Ie(n){let e=n.record,t=document.createElement("div");t.className="de-record-item de-verify-item";let r=document.createElement("div");r.className="de-record-header";let o=document.createElement("span");o.className="de-record-type",o.textContent=e.type,r.appendChild(o);let i=document.createElement("span");i.className="de-record-name",i.textContent=e.name,r.appendChild(i),t.appendChild(r);let s=document.createElement("div");switch(s.className="de-verify-status",n.status){case"verified":{let a=document.createElement("span");a.className="de-verify-check",N(a,Se),s.appendChild(a);let l=document.createElement("span");l.className="de-status-text de-verified",l.textContent="Verified",s.appendChild(l);break}case"failed":{let a=document.createElement("span");a.className="de-verify-fail",N(a,Le),s.appendChild(a);let l=document.createElement("span");l.className="de-status-text de-failed",l.textContent=n.error||"Failed",s.appendChild(l);break}case"pushing":{let a=document.createElement("span");a.className="de-status-dot de-pushing",s.appendChild(a);let l=document.createElement("span");l.className="de-status-text",l.textContent="Pushing...",s.appendChild(l);break}default:{let a=document.createElement("span");a.className="de-status-dot de-pending",s.appendChild(a);let l=document.createElement("span");l.className="de-status-text",l.textContent="Pending",s.appendChild(l);break}}return t.appendChild(s),t}var Pe='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>',ae='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>',de='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>',_e='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>',Fe={cloudflare:"https://dash.cloudflare.com/?to=/:account/:zone/dns/records",route53:"https://console.aws.amazon.com/route53/v2/hostedzones",godaddy:"https://dcc.godaddy.com/manage/dns",google_cloud_dns:"https://console.cloud.google.com/net-services/dns/zones"};function p(n,e,t){let r=document.createElement(n);if(e)for(let[o,i]of Object.entries(e))o==="className"?r.className=i:r.setAttribute(o,i);if(t)for(let o of t)typeof o=="string"?r.appendChild(document.createTextNode(o)):r.appendChild(o);return r}function S(n,e){let t=document.createElement("template");t.innerHTML=e;let r=t.content;n.appendChild(r)}var T=class{constructor(e,t){this.overlay=null;this.previouslyFocused=null;this.verifyPoller=null;this.pushController=null;this.handleKeydown=e=>{let t=e;if(t.key==="Escape"){t.preventDefault(),this.close();return}t.key==="Tab"&&this.trapFocus(t)};this.shadow=e,this.ctx={options:t,state:d.IDLE,results:t.records.map(r=>({record:r,status:"pending"})),errorMessage:"",detectionOutcome:null,dcDiscovery:null,providerLabel:"",providerSlug:"",verifyTimedOut:!1,pushErrorMessage:""}}open(){this.previouslyFocused=document.activeElement;let e=document.createElement("style");e.textContent=_,this.shadow.appendChild(e);let t=F(this.ctx.options.theme),r=M(t,this.ctx.options.accentColor);V(this.shadow.host,r),this.dispatch(c.OPEN),this.render(),requestAnimationFrame(()=>{requestAnimationFrame(()=>{this.overlay?.classList.add("de-visible")})}),this.shadow.host.addEventListener("keydown",this.handleKeydown),this.startDetection()}openForVerify(e,t){this.previouslyFocused=document.activeElement,this.ctx.results=e,this.ctx.providerLabel=t;let r=document.createElement("style");r.textContent=_,this.shadow.appendChild(r);let o=F(this.ctx.options.theme),i=M(o,this.ctx.options.accentColor);V(this.shadow.host,i),this.dispatch(c.DC_RETURN),this.render(),requestAnimationFrame(()=>{requestAnimationFrame(()=>{this.overlay?.classList.add("de-visible")})}),this.shadow.host.addEventListener("keydown",this.handleKeydown),this.startVerifyPolling()}close(){if(!this.overlay)return;this.stopVerifyPolling(),this.stopPushController(),this.overlay.classList.remove("de-visible");let e=()=>{for(this.shadow.host.removeEventListener("keydown",this.handleKeydown);this.shadow.firstChild;)this.shadow.removeChild(this.shadow.firstChild);this.overlay=null,this.previouslyFocused&&this.previouslyFocused instanceof HTMLElement&&this.previouslyFocused.focus(),this.ctx.options.onClose?.()};this.overlay.addEventListener("transitionend",e,{once:!0}),setTimeout(e,300)}dispatch(e){this.ctx.state=B(this.ctx.state,e)}async startDetection(){try{let e=await W(this.ctx.options);switch(this.ctx.detectionOutcome=e,e.type){case"dc":this.ctx.dcDiscovery=e.dcDiscovery||null,this.ctx.providerLabel=e.label||e.provider||"your provider",this.dispatch(c.DC_AVAILABLE);break;case"credential":this.ctx.providerLabel=e.label||e.provider||"",this.ctx.providerSlug=e.provider||"",this.dispatch(c.DETECTED);break;case"fallback":default:this.ctx.providerLabel=e.label||e.provider||"",this.dispatch(c.NO_CREDS);break}}catch{this.dispatch(c.NO_CREDS)}this.render()}async handleDCAutoSetup(){if(!this.ctx.dcDiscovery){this.dispatch(c.NO_CREDS),this.render();return}q(this.ctx.options,this.ctx.dcDiscovery,this.ctx.providerLabel);let e=Q(this.ctx.options.domain,this.ctx.options.returnUrl),t={apiUrl:this.ctx.options.apiUrl,apiKey:this.ctx.options.apiKey},r=this.ctx.dcDiscovery;if(!r){this.ctx.errorMessage="Domain Connect discovery data missing.",this.dispatch(c.ERROR),this.render();return}let o=await K(t,{domain:this.ctx.options.domain,provider_id:r.ProviderID,url_async_ux:r.URLAsyncUX||r.URLSyncUX,redirect_uri:e});if(!o.ok){this.dispatch(c.ERROR),this.ctx.errorMessage=o.error,this.render();return}window.location.href=o.data.url}handleDCManual(){this.dispatch(c.NO_CREDS),this.render()}startVerifyPolling(){this.stopVerifyPolling();for(let e of this.ctx.results)e.status==="pending"&&(e.status="pushing");this.verifyPoller=te(this.ctx.options,this.ctx.results,{onRecordVerified:e=>{this.render()},onAllVerified:()=>{this.dispatch(c.ALL_VERIFIED),Z(this.ctx.options.domain),this.ctx.options.onComplete?.(this.ctx.results),this.render()},onTimeout:()=>{this.ctx.verifyTimedOut=!0,this.stopVerifyPolling(),this.render()}})}stopVerifyPolling(){this.verifyPoller&&(this.verifyPoller.stop(),this.verifyPoller=null)}handleManualVerify(){this.ctx.verifyTimedOut=!1,this.startVerifyPolling(),this.render()}render(){for(this.overlay||(this.overlay=document.createElement("div"),this.overlay.className="de-overlay",this.overlay.setAttribute("role","dialog"),this.overlay.setAttribute("aria-modal","true"),this.overlay.setAttribute("aria-label","DNS record setup"),this.overlay.addEventListener("click",e=>{e.target===this.overlay&&this.close()}),this.shadow.appendChild(this.overlay));this.overlay.firstChild;)this.overlay.removeChild(this.overlay.firstChild);this.overlay.appendChild(this.buildCard()),this.verifyPoller||this.focusFirst()}buildCard(){let e=p("div",{className:"de-card",role:"document"});e.appendChild(this.buildHeader()),e.appendChild(this.buildBody());let t=this.buildFooter();return t&&e.appendChild(t),e}buildHeader(){let e=p("div",{className:"de-header"}),t=p("div");t.appendChild(p("h2",{},["Configure DNS"]));let r=p("div",{className:"de-header-sub"}),o=p("span",{className:"de-domain"},[this.ctx.options.domain]);r.appendChild(o),t.appendChild(r),e.appendChild(t);let i=p("button",{className:"de-close-btn","aria-label":"Close dialog",type:"button"});return S(i,Pe),i.addEventListener("click",()=>this.close()),e.appendChild(i),e}buildBody(){let e=p("div",{className:"de-body"});switch(this.ctx.state){case d.DETECTING:e.appendChild(U());break;case d.DC_FLOW:{let t=$(this.ctx.providerLabel,this.ctx.options.records.length,()=>this.handleDCAutoSetup(),()=>this.handleDCManual());t.appendChild(this.buildManualFallbackLink()),e.appendChild(t);break}case d.CREDENTIAL_FLOW:{let t=oe(this.ctx.providerSlug,{onSubmit:(r,o)=>this.handleCredentialSubmit(r,o),onNoCreds:()=>this.handleCredentialNoCreds()});t.appendChild(this.buildManualFallbackLink()),e.appendChild(t);break}case d.FALLBACK:e.appendChild(this.buildFallback());break;case d.PUSHING:e.appendChild(se(this.ctx.results,this.ctx.pushErrorMessage||null,this.ctx.pushErrorMessage?()=>this.handlePushRetry():null));break;case d.VERIFYING:e.appendChild(re(this.ctx.results,this.ctx.verifyTimedOut,()=>this.handleManualVerify()));break;case d.COMPLETE:e.appendChild(this.buildComplete());break;case d.ERROR:e.appendChild(this.buildError());break;default:e.appendChild(U());break}return e}buildFallback(){let e=p("div",{className:"de-fallback"}),t=this.ctx.providerLabel,r=p("div",{className:"de-fallback-intro"});if(t){r.appendChild(p("p",{className:"de-fallback-heading"},["Log in to "+t+" and add these DNS records:"]));let i=Fe[this.ctx.providerSlug||""];if(i){let s=p("a",{className:"de-fallback-provider-link",href:i,target:"_blank",rel:"noopener noreferrer"},["Open "+t+" DNS settings"]);r.appendChild(s)}}else r.appendChild(p("p",{className:"de-fallback-heading"},["Log in to your DNS provider and add these records:"]));e.appendChild(r);let o=p("div",{className:"de-records-list"});for(let i of this.ctx.results)o.appendChild(this.buildRecordCard(i));return e.appendChild(o),e}buildRecordCard(e){let t=e.record,r=p("div",{className:"de-record-item"}),o=p("div",{className:"de-record-header"});o.appendChild(p("span",{className:"de-record-type"},[t.type])),o.appendChild(p("span",{className:"de-record-name"},[t.name])),r.appendChild(o);let i=p("div",{className:"de-record-value"});i.appendChild(p("code",{},[t.content]));let s=p("button",{className:"de-copy-btn","aria-label":"Copy value",type:"button"});return S(s,ae),s.addEventListener("click",()=>this.handleCopy(s,t.content)),i.appendChild(s),r.appendChild(i),t.ttl&&r.appendChild(p("div",{className:"de-record-ttl"},["TTL: "+t.ttl])),r}buildManualFallbackLink(){let e=p("div",{className:"de-manual-link-wrap"}),t=p("button",{className:"de-btn de-btn-link de-manual-link",type:"button"},["Add records manually"]);return t.addEventListener("click",()=>{this.dispatch(c.NO_CREDS),this.render()}),e.appendChild(t),e}buildComplete(){let e=p("div",{className:"de-complete"}),t=p("div",{className:"de-check-circle"});S(t,de),e.appendChild(t),e.appendChild(p("div",{className:"de-complete-title"},["DNS configured"]));let r=p("div",{className:"de-complete-sub"});return r.appendChild(document.createTextNode("All records have been verified for ")),r.appendChild(p("span",{className:"de-domain"},[this.ctx.options.domain])),e.appendChild(r),e}buildError(){let e=p("div",{className:"de-error-view"}),t=p("div",{className:"de-error-icon"});return S(t,_e),e.appendChild(t),e.appendChild(p("div",{className:"de-error-title"},["Something went wrong"])),e.appendChild(p("div",{className:"de-error-message"},[this.ctx.errorMessage||"An unexpected error occurred. Please try again."])),e}buildFooter(){let e=p("div",{className:"de-footer"});switch(this.ctx.state){case d.FALLBACK:{let t=p("button",{className:"de-btn de-btn-secondary",type:"button"},["Cancel"]);t.addEventListener("click",()=>this.close()),e.appendChild(t);let r=p("button",{className:"de-btn de-btn-primary",type:"button"},["Verify records"]);return r.addEventListener("click",()=>this.handleVerify()),e.appendChild(r),e}case d.PUSHING:{let t=p("button",{className:"de-btn de-btn-secondary",type:"button"},["Cancel"]);if(t.addEventListener("click",()=>this.close()),e.appendChild(t),!this.ctx.pushErrorMessage){let r=p("button",{className:"de-btn de-btn-primary",type:"button",disabled:"true"},["Pushing..."]);e.appendChild(r)}return e}case d.VERIFYING:{let t=p("button",{className:"de-btn de-btn-secondary",type:"button"},["Cancel"]);if(t.addEventListener("click",()=>this.close()),e.appendChild(t),!this.ctx.verifyTimedOut){let r=p("button",{className:"de-btn de-btn-primary",type:"button",disabled:"true"},["Verifying..."]);e.appendChild(r)}return e}case d.COMPLETE:{let t=p("button",{className:"de-btn de-btn-primary",type:"button"},["Done"]);return t.addEventListener("click",()=>this.close()),e.appendChild(t),e}case d.ERROR:{let t=p("button",{className:"de-btn de-btn-secondary",type:"button"},["Close"]);t.addEventListener("click",()=>this.close()),e.appendChild(t);let r=p("button",{className:"de-btn de-btn-primary",type:"button"},["Try again"]);return r.addEventListener("click",()=>{this.ctx.errorMessage="",this.ctx.pushErrorMessage="",this.ctx.verifyTimedOut=!1,this.stopPushController(),this.ctx.results=this.ctx.options.records.map(o=>({record:o,status:"pending"})),this.ctx.state=d.IDLE,this.dispatch(c.OPEN),this.render(),this.startDetection()}),e.appendChild(r),e}default:return null}}async handleCopy(e,t){try{await navigator.clipboard.writeText(t)}catch{let r=document.createElement("textarea");r.value=t,r.style.position="fixed",r.style.opacity="0",document.body.appendChild(r),r.select(),document.execCommand("copy"),document.body.removeChild(r)}for(e.classList.add("de-copied");e.firstChild;)e.removeChild(e.firstChild);S(e,de),setTimeout(()=>{for(e.classList.remove("de-copied");e.firstChild;)e.removeChild(e.firstChild);S(e,ae)},2e3)}handleCredentialSubmit(e,t){this.ctx.pushErrorMessage="",this.ctx.providerSlug=e,this.ctx.results=this.ctx.options.records.map(r=>({record:r,status:"pending"})),this.dispatch(c.CREDS_SUBMITTED),this.stopPushController(),this.pushController=ie(this.ctx.options,e,t,this.ctx.results,{onRender:()=>this.render(),onAllDone:()=>{this.ctx.results.every(o=>o.status==="verified")?(this.dispatch(c.ALL_VERIFIED),this.ctx.options.onComplete?.(this.ctx.results)):(this.dispatch(c.ALL_VERIFIED),this.ctx.options.onComplete?.(this.ctx.results)),this.render()},onError:r=>{this.ctx.pushErrorMessage=r;for(let o of this.ctx.results)o.status==="pushing"&&(o.status="pending");this.render()}}),this.render()}handleCredentialNoCreds(){this.dispatch(c.NO_CREDS),this.render()}handlePushRetry(){this.stopPushController(),this.ctx.pushErrorMessage="",this.ctx.results=this.ctx.options.records.map(e=>({record:e,status:"pending"})),this.ctx.state=d.CREDENTIAL_FLOW,this.render()}stopPushController(){this.pushController&&(this.pushController.stop(),this.pushController=null)}handleVerify(){this.dispatch(c.PUSH_STARTED),this.startVerifyPolling(),this.render()}focusFirst(){requestAnimationFrame(()=>{this.shadow.querySelector('button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')?.focus()})}trapFocus(e){let t=Array.from(this.shadow.querySelectorAll('button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'));if(t.length===0)return;let r=t[0],o=t[t.length-1],i=this.shadow.activeElement;e.shiftKey?(i===r||!i)&&(e.preventDefault(),o.focus()):(i===o||!i)&&(e.preventDefault(),r.focus())}};var le="dns-entree-widget",w=null,D=null;function ce(n){w&&P(),D=document.createElement(le),document.body.appendChild(D);let e=D.attachShadow({mode:"open"});w=new T(e,n),w.open()}function pe(n,e,t){w&&P(),D=document.createElement(le),document.body.appendChild(D);let r=D.attachShadow({mode:"open"});w=new T(r,n),w.openForVerify(e,t)}function P(){let n=w,e=D;w=null,D=null,e&&e.remove()}function ue(){return w!==null}function at(n){if(!n.domain)throw new Error("DnsEntree.open(): domain is required");if(!n.records||n.records.length===0)throw new Error("DnsEntree.open(): at least one record is required");if(!n.apiUrl)throw new Error("DnsEntree.open(): apiUrl is required");ce(n)}function dt(){P()}function lt(){return ue()}function ct(n){let e=ee();if(!e)return!1;let t=J(e);if(!t)return H(),!1;H();let r={...n,domain:t.domain,records:t.records},o=t.records.map(i=>({record:i,status:"pending"}));return pe(r,o,t.providerLabel),!0}export{dt as close,ct as handleDCReturn,lt as isOpen,at as open};
