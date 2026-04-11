/** A DNS record to be configured. */
export interface DnsRecord {
  type: string;
  name: string;
  content: string;
  ttl?: number;
}

/** Per-record result returned to the caller. */
export interface RecordResult {
  record: DnsRecord;
  status: "pending" | "pushing" | "verified" | "failed";
  error?: string;
}

/** Options passed to DnsEntree.open(). */
export interface DnsEntreeOptions {
  domain: string;
  records: DnsRecord[];
  apiUrl: string;
  apiKey?: string;
  theme?: "light" | "dark" | "auto";
  accentColor?: string;
  returnUrl?: string;
  onComplete?: (results: RecordResult[]) => void;
  onError?: (error: Error) => void;
  onClose?: () => void;
}
