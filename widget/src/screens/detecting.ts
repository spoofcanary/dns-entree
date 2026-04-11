import { detect, dcDiscover } from "../api";
import type { ApiConfig, DetectData, DCDiscoverData } from "../api";
import type { DnsEntreeOptions } from "../types";

/** Detection result passed to the modal for state routing. */
export interface DetectionOutcome {
  type: "dc" | "credential" | "fallback";
  provider?: string;
  label?: string;
  dcDiscovery?: DCDiscoverData;
  detection?: DetectData;
}

/** Build the detecting screen DOM. */
export function buildDetecting(): HTMLElement {
  const wrap = document.createElement("div");
  wrap.className = "de-detecting";

  const spinner = document.createElement("div");
  spinner.className = "de-spinner";
  wrap.appendChild(spinner);

  const text = document.createElement("div");
  text.className = "de-detecting-text";
  text.textContent = "Checking your DNS provider...";
  wrap.appendChild(text);

  return wrap;
}

/**
 * Run provider detection + DC discovery in parallel.
 * Returns the appropriate flow tier.
 */
export async function runDetection(
  options: DnsEntreeOptions,
): Promise<DetectionOutcome> {
  const cfg: ApiConfig = { apiUrl: options.apiUrl, apiKey: options.apiKey };

  // Run detect and DC discover in parallel
  const [detectResult, dcResult] = await Promise.all([
    detect(cfg, options.domain),
    dcDiscover(cfg, options.domain),
  ]);

  // If DC is supported, that's the best path
  if (dcResult.ok && dcResult.data.Supported) {
    const label = dcResult.data.ProviderName || (detectResult.ok ? (detectResult as { ok: true; data: DetectData }).data.label : "");
    return {
      type: "dc",
      provider: dcResult.data.ProviderID,
      label: dcResult.data.ProviderName || label,
      dcDiscovery: dcResult.data,
      detection: detectResult.ok ? detectResult.data : undefined,
    };
  }

  // If provider detected with API support, credential flow
  if (detectResult.ok && detectResult.data.supported && detectResult.data.provider) {
    return {
      type: "credential",
      provider: detectResult.data.provider,
      label: detectResult.data.label,
      detection: detectResult.data,
    };
  }

  // If provider detected but no API support, or unknown - fallback
  if (detectResult.ok && detectResult.data.provider) {
    return {
      type: "fallback",
      provider: detectResult.data.provider,
      label: detectResult.data.label,
      detection: detectResult.data,
    };
  }

  // Complete fallback - nothing detected
  return { type: "fallback" };
}
