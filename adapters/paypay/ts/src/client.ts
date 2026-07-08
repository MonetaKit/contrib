// PayPay charges — the TS twin of the Go adapter's Charge. Replays the same
// vectors/gateway.json, so the two implementations cannot drift on how an OPA
// response classifies into the neutral ChargeResult.

import { authHeader } from "./auth.ts";

export type ChargeStatus = "succeeded" | "pending" | "failed";

export interface ChargeAction {
  type: string;
  value: string;
}

export interface ChargeResult {
  status: ChargeStatus;
  providerRef: string;
  action?: ChargeAction;
}

export interface Money {
  amount: number; // smallest currency unit (whole yen — JPY is zero-decimal)
  currency: string;
}

export interface ChargeOpts {
  idempotencyKey: string; // becomes merchantPaymentId (sanitized)
  description?: string;
}

export interface PayPayConfig {
  apiKey: string;
  apiSecret: string;
  merchantId?: string;
  /** default "production"; "sandbox" targets stg-api.sandbox.paypay.ne.jp */
  environment?: "production" | "sandbox";
  /** overrides environment; for tests */
  baseUrl?: string;
  fetch?: typeof fetch;
  /** epoch seconds; injectable for tests */
  now?: () => number;
  nonce?: () => string;
}

/** A non-2xx OPA response that is NOT a business decline (retryable path). */
export class PayPayError extends Error {
  readonly httpStatus: number;
  readonly code: string;
  readonly codeId: string;
  constructor(method: string, path: string, httpStatus: number, code: string, codeId: string, message: string) {
    super(`paypay: ${method} ${path} -> ${httpStatus} ${code} (codeId ${codeId}): ${message}`);
    this.httpStatus = httpStatus;
    this.code = code;
    this.codeId = codeId;
  }
}

const PRODUCTION_BASE_URL = "https://api.paypay.ne.jp";
const SANDBOX_BASE_URL = "https://stg-api.sandbox.paypay.ne.jp";

// Business outcomes (the wallet/user cannot pay) — ChargeResult "failed",
// which consumes a dunning attempt; everything else non-2xx throws for the
// scheduler's lease-retry path. Mirrors the Go adapter's declineCodes.
const DECLINE_CODES = new Set([
  "NO_SUFFICIENT_FUND",
  "LIMIT_EXCEEDED",
  "USER_DEFINED_DAILY_LIMIT_EXCEEDED",
  "USER_DEFINED_MONTHLY_LIMIT_EXCEEDED",
  "USER_DAILY_LIMIT_FOR_MERCHANT_EXCEEDED",
  "NON_KYC_USER",
  "CANCELED_USER",
  "NO_VALID_PAYMENT_METHOD",
  "CC_LIMIT_EXCEEDED",
  "PPC_EXPIRED",
  "PPC_LIMIT_EXCEEDED",
  "PAY_METHOD_INVALIDATED",
  "USER_STATE_IS_NOT_ACTIVE",
  "INVALID_USER_AUTHORIZATION_ID",
  "EXPIRED_USER_AUTHORIZATION_ID",
  "SUSPECTED_DUPLICATE_PAYMENT",
]);

interface Envelope {
  resultInfo: { code: string; message: string; codeId: string };
  data: any;
}

function chargeStatus(s: string): ChargeStatus | undefined {
  switch (s) {
    case "COMPLETED":
    case "REFUNDED": // idempotent replay of a charge refunded later
      return "succeeded";
    case "CREATED":
    case "AUTHORIZED":
    case "REAUTHORIZING":
      return "pending";
    case "FAILED":
    case "CANCELED":
    case "EXPIRED":
      return "failed";
  }
  return undefined;
}

const VALID_MPID = /^[a-zA-Z0-9_-]{1,64}$/;

/**
 * Makes an idempotency key valid for PayPay (≤64 chars of [a-zA-Z0-9_-]).
 * Invalid keys map deterministically to sanitized[:40] + "-" + sha256[:16] of
 * the ORIGINAL key — identical to the Go adapter's merchantPaymentID.
 */
export async function merchantPaymentId(key: string): Promise<string> {
  if (VALID_MPID.test(key)) return key;
  const sanitized = key.replace(/[^a-zA-Z0-9_-]/g, "-").slice(0, 40);
  const sum = new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(key)));
  const hex = [...sum].map((b) => b.toString(16).padStart(2, "0")).join("").slice(0, 16);
  return `${sanitized}-${hex}`;
}

export class PayPayClient {
  private readonly cfg: Required<Pick<PayPayConfig, "apiKey" | "apiSecret">> & PayPayConfig;
  private readonly baseUrl: string;
  private readonly fetchFn: typeof fetch;
  private readonly now: () => number;
  private readonly nonce: () => string;

  constructor(cfg: PayPayConfig) {
    this.cfg = cfg;
    this.baseUrl = cfg.baseUrl ?? (cfg.environment === "sandbox" ? SANDBOX_BASE_URL : PRODUCTION_BASE_URL);
    this.fetchFn = cfg.fetch ?? fetch;
    this.now = cfg.now ?? (() => Math.floor(Date.now() / 1000));
    this.nonce = cfg.nonce ?? (() => crypto.randomUUID());
  }

  /**
   * Collects `amount` (JPY only) from a PayPay user. token forms:
   *  - a userAuthorizationId (account link): off-session continuous payment
   *  - "qr": dynamic QR push payment — "pending" + payment URL in `action`
   */
  async charge(token: string, amount: Money, opts: ChargeOpts): Promise<ChargeResult> {
    if (amount.currency.toLowerCase() !== "jpy") {
      throw new Error(`paypay: only jpy is supported, got ${JSON.stringify(amount.currency)}`);
    }
    if (!opts.idempotencyKey) {
      throw new Error("paypay: opts.idempotencyKey is required (it becomes merchantPaymentId)");
    }
    if (token === "qr") return this.chargeQR(amount, opts);
    return this.chargeContinuous(token, amount, opts);
  }

  private async chargeContinuous(userAuthorizationId: string, amount: Money, opts: ChargeOpts): Promise<ChargeResult> {
    const path = "/v1/subscription/payments";
    // Key order is part of the signed bytes — keep in sync with the Go struct.
    const body: Record<string, unknown> = {
      merchantPaymentId: await merchantPaymentId(opts.idempotencyKey),
      userAuthorizationId,
      amount: { amount: amount.amount, currency: "JPY" },
      requestedAt: this.now(),
    };
    if (opts.description) body.orderDescription = opts.description;
    const { status, env } = await this.request("POST", path, body);
    const data = env.data ?? {};
    if (status >= 200 && status < 300) {
      const mapped = chargeStatus(data.status);
      if (!mapped) throw new Error(`paypay: unknown payment status ${JSON.stringify(data.status)} (paymentId ${data.paymentId})`);
      return { status: mapped, providerRef: data.paymentId ?? "" };
    }
    if (DECLINE_CODES.has(env.resultInfo.code)) {
      return { status: "failed", providerRef: data?.paymentId ?? "" };
    }
    throw new PayPayError("POST", path, status, env.resultInfo.code, env.resultInfo.codeId, env.resultInfo.message);
  }

  private async chargeQR(amount: Money, opts: ChargeOpts): Promise<ChargeResult> {
    const path = "/v2/codes";
    const mpid = await merchantPaymentId(opts.idempotencyKey);
    const body: Record<string, unknown> = {
      merchantPaymentId: mpid,
      amount: { amount: amount.amount, currency: "JPY" },
      codeType: "ORDER_QR",
      requestedAt: this.now(),
    };
    if (opts.description) body.orderDescription = opts.description;
    const { status, env } = await this.request("POST", path, body);
    if (status >= 200 && status < 300) {
      const data = env.data ?? {};
      if (!data.url) throw new Error("paypay: create code succeeded but no url in response");
      return {
        status: "pending",
        providerRef: data.codeId ?? "",
        action: { type: "paypay_qr", value: data.url },
      };
    }
    // /v2/codes does not replay duplicates — fall back to payment details.
    if (env.resultInfo.code === "DUPLICATE_DYNAMIC_QR_REQUEST") {
      return this.qrPaymentStatus(mpid);
    }
    throw new PayPayError("POST", path, status, env.resultInfo.code, env.resultInfo.codeId, env.resultInfo.message);
  }

  private async qrPaymentStatus(mpid: string): Promise<ChargeResult> {
    const path = `/v2/codes/payments/${mpid}`;
    const { status, env } = await this.request("GET", path, null);
    if (status < 200 || status >= 300) {
      throw new PayPayError("GET", path, status, env.resultInfo.code, env.resultInfo.codeId, env.resultInfo.message);
    }
    const data = env.data ?? {};
    const mapped = chargeStatus(data.status);
    if (!mapped) throw new Error(`paypay: unknown payment status ${JSON.stringify(data.status)}`);
    return { status: mapped, providerRef: data.paymentId ?? "" };
  }

  private async request(method: string, path: string, body: Record<string, unknown> | null): Promise<{ status: number; env: Envelope }> {
    const payload = body ? JSON.stringify(body) : null;
    const headers: Record<string, string> = {
      Authorization: await authHeader(this.cfg.apiKey, this.cfg.apiSecret, method, path, payload, this.nonce(), this.now()),
    };
    if (this.cfg.merchantId) headers["X-ASSUME-MERCHANT"] = this.cfg.merchantId;
    if (payload) headers["Content-Type"] = "application/json";
    const resp = await this.fetchFn(this.baseUrl + path, { method, headers, body: payload });
    let env: Envelope;
    try {
      env = (await resp.json()) as Envelope;
    } catch {
      throw new Error(`paypay: ${method} ${path} -> ${resp.status}: non-OPA response body`);
    }
    // JSON without the OPA envelope (a proxy's error JSON, say) would surface
    // later as a bare TypeError on env.resultInfo.code — same failure, same error.
    if (typeof env?.resultInfo?.code !== "string") {
      throw new Error(`paypay: ${method} ${path} -> ${resp.status}: non-OPA response body`);
    }
    return { status: resp.status, env };
  }
}

export function createClient(cfg: PayPayConfig): PayPayClient {
  return new PayPayClient(cfg);
}
