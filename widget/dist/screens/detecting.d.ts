import type { DetectData, DCDiscoverData } from "../api";
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
export declare function buildDetecting(): HTMLElement;
/**
 * Run provider detection + DC discovery in parallel.
 * Returns the appropriate flow tier.
 */
export declare function runDetection(options: DnsEntreeOptions): Promise<DetectionOutcome>;
