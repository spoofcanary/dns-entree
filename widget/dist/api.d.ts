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
/** POST /v1/detect */
export declare function detect(cfg: ApiConfig, domain: string): Promise<ApiResult<DetectData>>;
/** POST /v1/dc/discover */
export declare function dcDiscover(cfg: ApiConfig, domain: string): Promise<ApiResult<DCDiscoverData>>;
/** POST /v1/dc/apply-url */
export declare function dcApplyUrl(cfg: ApiConfig, opts: {
    domain: string;
    provider_id: string;
    url_async_ux: string;
    redirect_uri?: string;
    service_id?: string;
    host?: string;
    params?: Record<string, string>;
}): Promise<ApiResult<DCApplyUrlData>>;
/** POST /v1/verify */
export declare function verify(cfg: ApiConfig, domain: string, type: string, name: string, contains: string): Promise<ApiResult<VerifyData>>;
/** POST /v1/apply (with credential headers) */
export declare function apply(cfg: ApiConfig, provider: string, creds: Record<string, string>, domain: string, records: {
    type: string;
    name: string;
    content: string;
    ttl?: number;
}[]): Promise<ApiResult<ApplyData>>;
