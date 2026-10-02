import assert from "node:assert/strict";
import { test } from "node:test";
import { chromium } from "@playwright/test";

function required(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
if (
  new URL(origin).origin !== origin ||
  !origin.startsWith("https://127.0.0.1:")
)
  throw new Error("BRW05 requires the task-owned HTTPS loopback edge");
const phase = required("LC_BROWSER_INPUT_PHASE");
if (phase !== "stop" && phase !== "revoke")
  throw new Error("invalid BRW05 phase");
const store = required("LC_BROWSER_INPUT_STORE");
const unlistedStore = required("LC_BROWSER_INPUT_UNLISTED_STORE");
const session = required("LC_BROWSER_INPUT_SESSION");
const authorization = required("LC_BROWSER_INPUT_AUTHORIZATION");
const attempt = required("LC_BROWSER_INPUT_ATTEMPT");
const base = `/api/stores/${store}/live-sessions/${session}`;

test(
  `BRW05 ${phase}: genuine HTTPS cookie login and postcommit token`,
  { timeout: 110_000 },
  async () => {
    const browser = await chromium.launch({ headless: true });
    try {
      // Fixture CA is ephemeral. Only this local browser context ignores it;
      // production auth and transport settings are not relaxed.
      const context = await browser.newContext({
        baseURL: origin,
        ignoreHTTPSErrors: true,
      });
      const page = await context.newPage();
      await page.goto(`${origin}/en/`);
      await page
        .getByRole("button", { name: "Sign in with identity service" })
        .click();
      let cookies = await context.cookies();
      for (
        let i = 0;
        i < 100 &&
        !cookies.some((item) => item.name === "__Host-commerce_session");
        i++
      ) {
        await new Promise((resolve) => setTimeout(resolve, 50));
        cookies = await context.cookies();
      }
      const sessionCookie = cookies.find(
        (item) => item.name === "__Host-commerce_session",
      );
      const csrfCookie = cookies.find(
        (item) => item.name === "__Host-commerce_csrf",
      );
      assert.equal(sessionCookie?.secure, true);
      assert.equal(sessionCookie?.httpOnly, true);
      assert.equal(csrfCookie?.secure, true);
      assert.equal(csrfCookie?.httpOnly, false);

      // All credentials are attached by Chromium to same-origin fetches. JWTs
      // remain local to the page function; only safe booleans/statuses return.
      // G-UI8 audit [FIXTURE/SETUP]: token-transport probe: JWTs stay in the page function, only statuses/booleans return (the subject is the BFF/Go token chain, not a UI control)
      const result = await page.evaluate(
        async ({
          base,
          store,
          unlistedStore,
          session,
          authorization,
          attempt,
          phase,
        }) => {
          const csrfParts = document.cookie
            .split(";")
            .map((part) => part.trim())
            .filter((part) => part.startsWith("__Host-commerce_csrf="));
          const csrf =
            csrfParts.length === 1
              ? csrfParts[0].slice("__Host-commerce_csrf=".length)
              : "";
          const request = async (
            path: string,
            method: string,
            body?: string,
            key?: string,
            includeCSRF = true,
          ) => {
            const headers = new Headers();
            if (body !== undefined)
              headers.set("Content-Type", "application/json");
            if (key) headers.set("Idempotency-Key", key);
            if (includeCSRF && method !== "GET" && csrf)
              headers.set("X-CSRF-Token", csrf);
            const response = await fetch(path, {
              method,
              body,
              headers,
              credentials: "same-origin",
            });
            const raw = await response.text();
            let parsed: Record<string, unknown> | null = null;
            try {
              parsed = JSON.parse(raw) as Record<string, unknown>;
            } catch {
              /* failure is a safe fact below */
            }
            return {
              status: response.status,
              cache: response.headers.get("cache-control"),
              leakedHeaders:
                response.headers.has("x-backend-secret") ||
                response.headers.has("set-cookie"),
              raw,
              parsed,
            };
          };
          const startPath = `${base}/input/start`;
          const tokenPath = `${base}/input/token`;
          const inputPath = `${base}/input`;
          const beforeInput = await request(inputPath, "GET");
          const beforePrepared = await request(`${inputPath}/prepared`, "GET");
          const candidate = beforePrepared.parsed;
          const readPreparedOK = beforeInput.status === 200 && beforeInput.parsed === null &&
            beforeInput.cache === "private, no-store" && beforePrepared.status === 200 &&
            beforePrepared.cache === "private, no-store" && !!candidate &&
            Object.keys(candidate).sort().join(",") === "authorization_id,destinations,environment,session_version,start_before" &&
            candidate.authorization_id === authorization && candidate.session_version === 1 && candidate.environment === "MOCK";
          const invalidReads: number[] = [];
          const sanitation: boolean[] = [];
          for (const suffix of ["/input", "/input/prepared"]) {
            invalidReads.push((await request(`${base}${suffix}?x=1`, "GET")).status);
            invalidReads.push((await request(`${base}${suffix}`, "GET", undefined, "invalid-read-key")).status);
            invalidReads.push((await request(`${base}${suffix}`, "POST", "{}", "invalid-read-method")).status);
            invalidReads.push((await request(`/api/stores/${unlistedStore}/live-sessions/${session}${suffix}`, "GET")).status);
            for (const invalidID of ["aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"]) {
              const bad = await request(`/api/stores/${store}/live-sessions/${invalidID}${suffix}`, "GET");
              sanitation.push(bad.status === 503 && bad.cache === "private, no-store" &&
                !bad.raw.includes("upstream-secret-sentinel") && !bad.leakedHeaders);
            }
          }
          const startBody = JSON.stringify({
            authorization_id: authorization,
            expected_session_version: 1,
          });
          const tokenBody = JSON.stringify({
            attempt_id: attempt,
            expected_session_version: 1,
          });
          const startKey = `brw05-${phase}-start`;
          const tokenKey = `brw05-${phase}-token`;
          const started = await request(startPath, "POST", startBody, startKey);
          const startReplay = await request(
            startPath,
            "POST",
            startBody,
            startKey,
          );
          const receipt = started.parsed;
          const receiptOK =
            started.status === 200 &&
            startReplay.status === 200 &&
            !!receipt &&
            JSON.stringify(started.parsed) ===
              JSON.stringify(startReplay.parsed) &&
            Object.keys(receipt).sort().join(",") ===
              "attempt_id,session_id,state" &&
            receipt.attempt_id === attempt &&
            receipt.session_id === session &&
            !started.raw.includes("token") &&
            started.cache === "private, no-store";
          const first = await request(tokenPath, "POST", tokenBody, tokenKey);
          const replay = await request(tokenPath, "POST", tokenBody, tokenKey);
          const statusRead = await request(inputPath, "GET");
          const afterPrepared = await request(`${inputPath}/prepared`, "GET");
          const inputState = statusRead.parsed;
          const readInputOK = statusRead.status === 200 && statusRead.cache === "private, no-store" && !!inputState &&
            Object.keys(inputState).sort().join(",") === "admission_closed,attempt_id,can_stop,cleanup_held,close_reason,state,updated_at" &&
            inputState.attempt_id === attempt && inputState.state === "RESERVED" && inputState.can_stop === true &&
            inputState.admission_closed === false && inputState.close_reason === "" && inputState.cleanup_held === false &&
            afterPrepared.status === 200 && afterPrepared.parsed === null && afterPrepared.cache === "private, no-store";
          const detail = await request(base, "GET");
          const legacyForwardOK = detail.status === 200 && detail.cache === "private, no-store" &&
            detail.parsed?.prepared === null &&
            (detail.parsed?.attempt as Record<string, unknown> | undefined)?.attempt_id === attempt;
          const grant = first.parsed;
          const replayGrant = replay.parsed;
          const token = typeof grant?.token === "string" ? grant.token : "";
          const tokenOK =
            first.status === 200 &&
            replay.status === 200 &&
            !!grant &&
            !!replayGrant &&
            Object.keys(grant).sort().join(",") ===
              "attempt_id,expires_at,publisher_identity,room_name,token,url" &&
            new TextEncoder().encode(first.raw).byteLength <= 8192 &&
            first.cache === "private, no-store" &&
            grant.attempt_id === attempt &&
            grant.room_name === `lc_${attempt.replaceAll("-", "")}` &&
            typeof grant.publisher_identity === "string" &&
            /^lcp_[0-9a-f]{32}$/.test(grant.publisher_identity) &&
            typeof grant.expires_at === "number" &&
            Number.isSafeInteger(grant.expires_at) &&
            typeof grant.url === "string" &&
            grant.url.startsWith("wss://127.0.0.1:") &&
            /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(token) &&
            replayGrant.expires_at === grant.expires_at &&
            replayGrant.token === token &&
            replayGrant.url === grant.url;
          const storageClean = [localStorage, sessionStorage].every((storage) =>
            Array.from(
              { length: storage.length },
              (_, i) => storage.getItem(storage.key(i) ?? "") ?? "",
            ).every((value) => !token || !value.includes(token)),
          );
          // R4S-04: this document's connect-src is 'self', so the browser must refuse a WebSocket from it to the SFU origin and report the
          // violation. A publisher UI that needs the SFU must add that origin to the policy deliberately (then this assertion is updated with it).
          const sfuURL = typeof grant?.url === "string" ? (grant.url as string) : "";
          const wssBlockedByCSP =
            sfuURL !== "" &&
            (await new Promise<boolean>((resolve) => {
              const timer = setTimeout(() => resolve(false), 3000);
              document.addEventListener(
                "securitypolicyviolation",
                (event) => {
                  clearTimeout(timer);
                  resolve(event.violatedDirective.startsWith("connect-src"));
                },
                { once: true },
              );
              new WebSocket(sfuURL).close();
            }));

          const csrfDenied = await request(
            tokenPath,
            "POST",
            tokenBody,
            `brw05-${phase}-csrf`,
            false,
          );
          const wrongStore = await request(
            `/api/stores/${unlistedStore}/live-sessions/${session}/input/token`,
            "POST",
            tokenBody,
            `brw05-${phase}-store`,
          );
          const wrongMethod = await request(startPath, "GET");
          const query = await request(
            `${tokenPath}?x=1`,
            "POST",
            tokenBody,
            `brw05-${phase}-query`,
          );
          const malformed = await request(
            tokenPath,
            "POST",
            tokenBody,
            "brw05-malformed",
          );
          const oversized = await request(
            tokenPath,
            "POST",
            tokenBody,
            "brw05-oversized",
          );
          return {
            csrfVisible: /^[A-Za-z0-9_-]{43}$/.test(csrf),
            receiptOK,
            readPreparedOK,
            readInputOK,
            legacyForwardOK,
            invalidReads,
            sanitation,
            tokenOK,
            storageClean,
            sfuURL,
            wssBlockedByCSP,
            csrfStatus: csrfDenied.status,
            storeStatus: wrongStore.status,
            methodStatus: wrongMethod.status,
            queryStatus: query.status,
            malformedStatus: malformed.status,
            oversizedStatus: oversized.status,
            malformedSafe:
              !malformed.raw.includes("upstream-secret-sentinel") &&
              !malformed.raw.includes("token") &&
              !malformed.leakedHeaders &&
              malformed.cache === "private, no-store",
            oversizedSafe:
              !oversized.raw.includes("token") &&
              !oversized.leakedHeaders &&
              oversized.cache === "private, no-store",
          };
        },
        { base, store, unlistedStore, session, authorization, attempt, phase },
      );
      assert.equal(result.csrfVisible, true);
      assert.equal(result.receiptOK, true);
      assert.equal(result.readPreparedOK, true);
      assert.equal(result.readInputOK, true);
      assert.equal(result.legacyForwardOK, true);
      assert.deepEqual(result.invalidReads, [422, 422, 405, 404, 422, 422, 405, 404]);
      assert.deepEqual(result.sanitation, [true, true, true, true]);
      assert.equal(result.tokenOK, true);
      assert.equal(result.storageClean, true);
      assert.equal(result.wssBlockedByCSP, true);
      // The SFU transport itself (TLS + WebSocket upgrade in a real Chromium) is proven from a CSP-free document, with the URL the grant returned.
      assert.match(result.sfuURL, /^wss:\/\/127\.0\.0\.1:\d+\/?$/);
      const sfuProbe = await context.newPage();
      const wssOpened = await sfuProbe.evaluate(
        (url) =>
          new Promise<boolean>((resolve) => {
            const socket = new WebSocket(url);
            const timer = setTimeout(() => {
              socket.close();
              resolve(false);
            }, 3000);
            socket.onopen = () => {
              clearTimeout(timer);
              socket.close();
              resolve(true);
            };
            socket.onerror = () => {
              clearTimeout(timer);
              resolve(false);
            };
          }),
        result.sfuURL,
      );
      await sfuProbe.close();
      assert.equal(wssOpened, true);
      assert.equal(result.csrfStatus, 403);
      assert.equal(result.storeStatus, 404);
      assert.equal(result.methodStatus, 405);
      assert.equal(result.queryStatus, 422);
      assert.equal(result.malformedStatus, 503);
      assert.equal(result.oversizedStatus, 503);
      assert.equal(result.malformedSafe, true);
      assert.equal(result.oversizedSafe, true);
      assert.equal(
        (await context.cookies()).some((item) => item.name === "upstream"),
        false,
      );

      // BrowserContext.request shares Chromium's issued cookie jar. No Cookie
      // or bearer header is injected; vary only Origin with a live grant/key.
      const probe = `${origin}${base}/input/token`;
      const probeBody = JSON.stringify({
        attempt_id: attempt,
        expected_session_version: 1,
      });
      const headers = {
        "Content-Type": "application/json",
        "X-CSRF-Token": csrfCookie.value,
        "Idempotency-Key": `brw05-origin-${phase}`,
      };
      // G-UI8 audit [FIXTURE/SETUP]: negative probe: a forged/hostile request no UI can send; the server, not the UI, must refuse (UI click paths of the same route are covered elsewhere) (forged Origin)
      const control = await context.request.post(probe, {
        headers: { ...headers, Origin: origin },
        data: probeBody,
      });
      assert.equal(control.status(), 200);
      // G-UI8 audit [FIXTURE/SETUP]: negative probe: a forged/hostile request no UI can send; the server, not the UI, must refuse (UI click paths of the same route are covered elsewhere) (forged Origin)
      const badOrigin = await context.request.post(probe, {
        headers: { ...headers, Origin: "https://attacker.invalid" },
        data: probeBody,
      });
      assert.equal(badOrigin.status(), 403);

      // G-UI8 audit [FIXTURE/SETUP]: negative probe: a forged/hostile request no UI can send; the server, not the UI, must refuse (UI click paths of the same route are covered elsewhere) (closed-session replay
      const closed = await page.evaluate(
        async ({ base, attempt, phase }) => {
          const csrf =
            document.cookie
              .split(";")
              .map((part) => part.trim())
              .find((part) => part.startsWith("__Host-commerce_csrf="))
              ?.split("=")[1] ?? "";
          const request = async (path: string, key: string, body: string) => {
            const response = await fetch(path, {
              method: "POST",
              credentials: "same-origin",
              headers: {
                "Content-Type": "application/json",
                "Idempotency-Key": key,
                "X-CSRF-Token": csrf,
              },
              body,
            });
            const raw = await response.text();
            return {
              status: response.status,
              safe:
                !raw.includes("token") &&
                response.headers.get("cache-control") === "private, no-store",
            };
          };
          const status =
            phase === "stop"
              ? (
                  await request(
                    `${base}/rehearsal/stop`,
                    "brw05-stop-request",
                    JSON.stringify({ attempt_id: attempt }),
                  )
                ).status
              : (
                  await fetch("/__test/revoke-input", {
                    method: "POST",
                    credentials: "same-origin",
                  })
                ).status;
          const denied = await request(
            `${base}/input/token`,
            `brw05-${phase}-token`,
            JSON.stringify({
              attempt_id: attempt,
              expected_session_version: 1,
            }),
          );
          const response = await fetch(`${base}/input`, { credentials: "same-origin" });
          const input = await response.json();
          const readAfterClosureOK = response.status === 200 && response.headers.get("cache-control") === "private, no-store" &&
            input.attempt_id === attempt && (phase === "stop"
              ? input.admission_closed === true && input.close_reason === "merchant_stop" && input.can_stop === false
              : input.state === "RESERVED" && input.admission_closed === false && input.can_stop === true);
          return { status, denied, readAfterClosureOK };
        },
        { base, attempt, phase },
      );
      assert.equal(closed.status, phase === "stop" ? 200 : 204);
      assert.equal(closed.denied.status, 409);
      assert.equal(closed.denied.safe, true);
      assert.equal(closed.readAfterClosureOK, true);
    } finally {
      await browser.close();
    }
  },
);
