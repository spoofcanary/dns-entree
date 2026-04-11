# Embeddable DNS Widget

Drop-in JavaScript widget that detects your customer's DNS provider, applies records via Domain Connect or API credentials, and falls back to copy-paste instructions -- all inside a single modal.

## Installation

### npm

```sh
npm install @dns-entree/widget
```

```js
import { open, handleDCReturn } from "@dns-entree/widget";
```

### CDN (script tag)

```html
<script src="https://unpkg.com/@dns-entree/widget@0.1.0/dist/widget.js"
        integrity="sha384-rechRu251oo6Vw3JU0Mwu0idaavS8bsdFeKwJombJBLPaguIIF9wB8QWedJaRgsT"
        crossorigin="anonymous"></script>
```

The IIFE build exposes a global `DnsEntree` object.

For enterprise deployments, self-host `widget.js` from your own origin instead of using a third-party CDN. The file is self-contained with no external dependencies.

## Quick Start

Ten lines to a working DNS setup flow:

```html
<script src="https://unpkg.com/@dns-entree/widget@0.1.0/dist/widget.js"></script>
<button id="setup-dns">Set up DNS</button>
<script>
  document.getElementById("setup-dns").addEventListener("click", function () {
    DnsEntree.open({
      domain: "example.com",
      apiUrl: "http://localhost:8080",
      records: [
        { type: "TXT", name: "_dmarc.example.com", content: "v=DMARC1; p=reject;" }
      ],
    });
  });
</script>
```

The widget opens, detects the DNS provider, and guides the customer through the best available setup method.

## How It Works

The widget auto-detects which tier applies and renders the appropriate flow:

1. **Tier 1 - Domain Connect**: Zero credentials. The widget discovers DC support, redirects the customer to their DNS provider's consent screen, and polls for verification on return. Works with Cloudflare, Name.com, OVH, 1&1, and dozens more.

2. **Tier 2 - API Credentials**: Customer pastes their provider API token into the widget. Records are pushed via entree-api. Credentials are sent per-request and never stored.

3. **Tier 3 - Copy-Paste Fallback**: No credentials, no DC. The widget shows each record as a card with a copy button and provider-specific instructions. Works with zero backend calls -- records come from the array you passed in.

## Full Options Reference

```typescript
interface DnsEntreeOptions {
  /** The domain to configure DNS for. Required. */
  domain: string;

  /** DNS records to set up. Required, at least one. */
  records: DnsRecord[];

  /** URL of the entree-api server. Required. */
  apiUrl: string;

  /** API key for hosted entree-api. Optional for self-hosted. */
  apiKey?: string;

  /** Theme: "light", "dark", or "auto" (matches system). Default: "auto". */
  theme?: "light" | "dark" | "auto";

  /** Custom accent color (any CSS color value). Default: #2563eb. */
  accentColor?: string;

  /** URL to return to after Domain Connect redirect. Default: current page. */
  returnUrl?: string;

  /** Called when all records are verified. */
  onComplete?: (results: RecordResult[]) => void;

  /** Called on unrecoverable error. */
  onError?: (error: Error) => void;

  /** Called when the modal is closed. */
  onClose?: () => void;
}

interface DnsRecord {
  type: string;    // "TXT", "CNAME", "MX", "A", etc.
  name: string;    // Full record name: "_dmarc.example.com"
  content: string; // Record value
  ttl?: number;    // Optional TTL in seconds
}
```

## Theming

The widget auto-detects light/dark mode from `prefers-color-scheme` by default.

### Force a theme

```js
DnsEntree.open({
  // ...
  theme: "dark",
});
```

### Custom accent color

```js
DnsEntree.open({
  // ...
  accentColor: "#e11d48",
});
```

The accent color applies to buttons, badges, links, and focus rings. The widget derives all other colors from the base light/dark palette.

### Style isolation

All widget styles live inside a Shadow DOM. They cannot leak into your page and your page styles cannot affect the widget.

## Multi-Record Example

Pass multiple records and the widget handles them as a batch:

```js
DnsEntree.open({
  domain: "example.com",
  apiUrl: "https://your-entree-api.example.com",
  records: [
    { type: "TXT", name: "example.com", content: "v=spf1 include:_spf.provider.com ~all" },
    { type: "CNAME", name: "sc._domainkey.example.com", content: "sc._domainkey.provider.com" },
    { type: "TXT", name: "_dmarc.example.com", content: "v=DMARC1; p=reject; rua=mailto:dmarc@provider.com" },
  ],
  onComplete(results) {
    console.log("All records verified:", results);
  },
});
```

In the DC flow, all records are bundled into a single consent screen when possible. In the credential flow, records are pushed sequentially with per-record progress indicators. In the fallback, each record is shown as a separate card with its own copy button.

## Domain Connect Flow

When the widget detects that a domain's DNS host supports Domain Connect:

1. Widget calls `/v1/dc/discover` to confirm DC support
2. Shows "Set up automatically" button with the provider name
3. On click, builds a signed apply URL via `/v1/dc/apply-url`
4. Redirects to the DNS provider's consent screen (same tab)
5. Provider redirects back with `de_state` URL parameter
6. Widget reopens in verify mode and polls `/v1/verify` until records propagate

### Handling the return

Call `handleDCReturn()` on page load to catch the redirect:

```js
// On page load
var handled = DnsEntree.handleDCReturn({
  apiUrl: "http://localhost:8080",
  onComplete: function (results) {
    console.log("DNS configured:", results);
  },
});
// handled === true if a DC return was detected
```

This restores the saved widget state from sessionStorage and opens the modal directly into verification mode.

## Self-Hosted vs Hosted

### Self-hosted (free)

Run your own entree-api server. Full functionality, no restrictions.

```sh
go install github.com/spoofcanary/dns-entree/cmd/entree-api@latest
entree-api --listen :8080
```

Point the widget at your server:

```js
DnsEntree.open({
  apiUrl: "http://localhost:8080",
  // ...
});
```

### Hosted

Use the hosted entree-api at a managed URL. Pass your API key:

```js
DnsEntree.open({
  apiUrl: "https://api.dns-entree.dev",
  apiKey: "your-api-key",
  // ...
});
```

The widget works identically in both modes -- only the `apiUrl` differs.

## Security Model

- **Shadow DOM isolation**: Widget styles and DOM are fully encapsulated. No CSS leaks in or out.
- **Credentials never stored**: API tokens entered in the credential flow are sent per-request to entree-api, then cleared from memory. The widget holds no credential state.
- **Per-request auth**: entree-api is stateless. Each request carries its own provider credentials in headers. The server holds nothing between requests.
- **No cookies**: All API calls use `credentials: "omit"`. No cookies are sent or received.
- **CORS safe**: entree-api sets `Access-Control-Allow-Origin: *` since auth is via header, not cookies.
- **Fallback is offline**: Tier 3 copy-paste works with zero network calls. Records render from the array you passed to `open()`.

## API Reference

### `DnsEntree.open(options)`

Open the widget modal. Starts provider detection and renders the appropriate flow.

### `DnsEntree.close()`

Close and destroy the widget.

### `DnsEntree.isOpen()`

Returns `true` if the widget is currently open.

### `DnsEntree.handleDCReturn(options)`

Check for a Domain Connect return redirect and auto-open the widget in verify mode. Call on page load. Returns `true` if a return was detected.

The `options` parameter is the same as `open()` except `domain` and `records` are restored from saved state.

## Bundle Size

The widget ships as:

- `dist/widget.js` -- IIFE build for `<script>` tags (exposes `DnsEntree` global)
- `dist/widget.mjs` -- ESM build for bundlers
- `dist/widget.d.ts` -- TypeScript declarations

Target: under 20 KB gzipped. Zero runtime dependencies.
