/** Thin fetch wrapper for entree-api. */

export interface ApiConfig {
  apiUrl: string;
  apiKey?: string;
}

export interface ApiOk<T> {
  ok: true;
  data: T;
}

export interface ApiErr {
  ok: false;
  error: string;
  code?: string;
  details?: Record<string, unknown>;
}

export type ApiResult<T> = ApiOk<T> | ApiErr;

// --- Response types matching entree-api JSON envelopes ---

export interface DetectData {
  provider: string;
  label: string;
  supported: boolean;
  nameservers: string[];
  method: string;
}

export interface DCDiscoverData {
  Supported: boolean;
  ProviderID: string;
  ProviderName: string;
  URLSyncUX: string;
  URLAsyncUX: string;
  URLAPI: string;
  Width: number;
  Height: number;
  Nameservers: string[];
}

export interface DCApplyUrlData {
  url: string;
}

export interface VerifyData {
  verified: boolean;
  current_value: string;
  method: string;
  nameservers_queried: string[];
}

export interface ApplyResultEntry {
  type: string;
  name: string;
  status: string;
  record_value: string;
  previous_value?: string;
  verified: boolean;
  verify_error?: string;
}

export interface ApplyData {
  domain: string;
  dry_run: boolean;
  results: ApplyResultEntry[];
}

// --- API client ---

async function request<T>(
  cfg: ApiConfig,
  method: string,
  path: string,
  body?: unknown,
): Promise<ApiResult<T>> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (cfg.apiKey) {
    headers["Authorization"] = "Bearer " + cfg.apiKey;
  }

  let resp: Response;
  try {
    resp = await fetch(cfg.apiUrl.replace(/\/+$/, "") + path, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      credentials: "omit",
    });
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : "Network error" };
  }

  let json: unknown;
  try {
    json = await resp.json();
  } catch {
    return { ok: false, error: "Invalid JSON response" };
  }

  const envelope = json as Record<string, unknown>;

  if (envelope.ok === true && envelope.data !== undefined) {
    return { ok: true, data: envelope.data as T };
  }

  // Error envelope: { ok: false, error: { code, message, details } }
  const errPayload = envelope.error as Record<string, unknown> | undefined;
  if (errPayload) {
    return {
      ok: false,
      error: (errPayload.message as string) || "Unknown error",
      code: errPayload.code as string | undefined,
      details: errPayload.details as Record<string, unknown> | undefined,
    };
  }

  return { ok: false, error: "Unexpected response format" };
}

/** POST /v1/detect */
export function detect(cfg: ApiConfig, domain: string): Promise<ApiResult<DetectData>> {
  return request<DetectData>(cfg, "POST", "/v1/detect", { domain });
}

/** POST /v1/dc/discover */
export function dcDiscover(cfg: ApiConfig, domain: string): Promise<ApiResult<DCDiscoverData>> {
  return request<DCDiscoverData>(cfg, "POST", "/v1/dc/discover", { domain });
}

/** POST /v1/dc/apply-url */
export function dcApplyUrl(
  cfg: ApiConfig,
  opts: {
    domain: string;
    provider_id: string;
    url_async_ux: string;
    redirect_uri?: string;
    service_id?: string;
    host?: string;
    params?: Record<string, string>;
  },
): Promise<ApiResult<DCApplyUrlData>> {
  return request<DCApplyUrlData>(cfg, "POST", "/v1/dc/apply-url", opts);
}

/** POST /v1/verify */
export function verify(
  cfg: ApiConfig,
  domain: string,
  type: string,
  name: string,
  contains: string,
): Promise<ApiResult<VerifyData>> {
  return request<VerifyData>(cfg, "POST", "/v1/verify", {
    domain,
    type,
    name,
    contains,
  });
}

/** POST /v1/apply (with credential headers) */
export function apply(
  cfg: ApiConfig,
  provider: string,
  creds: Record<string, string>,
  domain: string,
  records: { type: string; name: string; content: string; ttl?: number }[],
): Promise<ApiResult<ApplyData>> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "X-Entree-Provider": provider,
  };
  if (cfg.apiKey) {
    headers["Authorization"] = "Bearer " + cfg.apiKey;
  }
  // Spread credential headers directly
  for (const [k, v] of Object.entries(creds)) {
    headers[k] = v;
  }

  return fetch(cfg.apiUrl.replace(/\/+$/, "") + "/v1/apply", {
    method: "POST",
    headers,
    body: JSON.stringify({ domain, records }),
    credentials: "omit",
  })
    .then((resp) => resp.json())
    .then((json: unknown) => {
      const envelope = json as Record<string, unknown>;
      if (envelope.ok === true && envelope.data !== undefined) {
        return { ok: true as const, data: envelope.data as ApplyData };
      }
      const errPayload = envelope.error as Record<string, unknown> | undefined;
      if (errPayload) {
        return {
          ok: false as const,
          error: (errPayload.message as string) || "Unknown error",
          code: errPayload.code as string | undefined,
          details: errPayload.details as Record<string, unknown> | undefined,
        };
      }
      return { ok: false as const, error: "Unexpected response format" };
    })
    .catch((e) => ({
      ok: false as const,
      error: e instanceof Error ? e.message : "Network error",
    }));
}
