// Purpose: share a context-bound cart and same-receipt recovery across the storefront shell.
// Depends on: React, buyer-client session authority, purchase cart/journal helpers and cart-state.
// Used by: storefront header, cart drawer, product purchase and cart pages; never a second cart writer.
"use client";

// Cart state of the whole storefront shell: the header count, the drawer, the product page "Add to cart" and the cart page
// all share this one provider. BFF routes: GET /api/buyer/session, POST session/prepare|activate (via lib/buyer-client.ts),
// GET/PUT /api/buyer/cart -> Go /v1/buyer/session, /v1/buyer/cart (internal/storefront cart; the EXISTING multi-SKU cart,
// there is no second cart). The bearer stays in the HttpOnly cookie; this file never reads it.
// Behaviour that matters:
// - Viewing a page never creates a session: the provider only READS the session and, when it is active, the cart. A buyer
//   session (and its cookie) is created at the first add-to-cart (initializeBuyerSession), so crawlers and bounces cost nothing.
// - Every write goes through writePurchase (journal + Idempotency-Key + CAS on the cart version the buyer saw), so a lost
//   response is retried with the same key ("uncertain" -> Retry), never duplicated.
// - A known order locator (knownOrderID) blocks cart writes exactly as the old product page did: the buyer is offered
//   "view my order" or "start a new cart" (continueShopping), neither cancels the order.
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import { initializeBuyerSession, readBuyerSession } from "../lib/buyer-client";
import { cartWithQuantity, continueShopping, knownOrderID, readPurchase, validCart, writePurchase } from "../lib/purchase";
import type { Cart } from "../lib/purchase";
import { cartCount, cartProblem } from "../lib/cart-state";
import type { CartProblem } from "../lib/cart-state";

type CartApi = {
  cart: Cart | null;
  context: string;
  count: number;
  ready: boolean;
  busy: boolean;
  problem: CartProblem | null;
  orderID: string | null;
  drawerOpen: boolean;
  setDrawerOpen: (open: boolean) => void;
  add: (sku: string, quantity: number) => Promise<boolean>;
  setQuantity: (sku: string, quantity: number) => Promise<boolean>;
  refresh: () => Promise<void>;
  retry: () => Promise<void>;
  startNewCart: () => Promise<void>;
  dismiss: () => void;
};

const Ctx = createContext<CartApi | null>(null);

/** Read the shell's current cart and explicitly invoked purchase commands. */
export function useCart(): CartApi {
  const value = useContext(Ctx);
  if (!value) throw new Error("useCart outside CartProvider");
  return value;
}

/** Keeps visible cart ownership aligned with the context used by the durable purchase journal. */
export default function CartProvider({ children }: { children: ReactNode }) {
  const [cart, setCart] = useState<Cart | null>(null);
  const [context, setContext] = useState("");
  const [ready, setReady] = useState(false);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<CartProblem | null>(null);
  const [orderID, setOrderID] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const working = useRef(false);
  const currentContext = useRef("");
  const adoptContext = useCallback((next: string) => {
    // Another tab may rotate the buyer. Never publish B with A's items/order,
    // even when B's read fails or its write acknowledgement is lost.
    if (currentContext.current !== next) {
      currentContext.current = next;
      setCart(null);
      setOrderID(null);
    }
    setContext(next);
  }, []);

  // Same-context read failures retain the last cart; a newly observed context starts empty.
  const refresh = useCallback(async () => {
    try {
      const session = await readBuyerSession();
      if (session.state === "active" && session.context) {
        adoptContext(session.context);
        const current = await readPurchase("cart", session.context, validCart);
        if (currentContext.current !== session.context) return;
        setCart(current);
        setOrderID(knownOrderID(session.context));
      } else {
        adoptContext("");
        setCart(null);
        setOrderID(null);
      }
    } catch {
      /* count stays as it was */
    } finally {
      setReady(true);
    }
  }, [adoptContext]);

  useEffect(() => {
    void refresh();
    const onFocus = () => {
      if (!working.current) void refresh();
    };
    // Another tab added, checked out or reset: its journal/locator writes land in localStorage.
    const onStorage = (event: StorageEvent) => {
      if (event.key?.startsWith("commerce-purchase-") && !working.current) void refresh();
    };
    window.addEventListener("focus", onFocus);
    window.addEventListener("storage", onStorage);
    return () => {
      window.removeEventListener("focus", onFocus);
      window.removeEventListener("storage", onStorage);
    };
  }, [refresh]);

  // One serialized mutation: make sure a session exists, read the CURRENT cart (fresh CAS version), compute the write, send.
  const mutate = useCallback(
    async (build: (current: Cart) => ReturnType<typeof cartWithQuantity>): Promise<boolean> => {
      if (working.current) return false;
      working.current = true;
      setBusy(true);
      setProblem(null);
      try {
        const session = await initializeBuyerSession();
        if (!session.context) throw new Error("no context");
        const ctx = session.context;
        const known = knownOrderID(ctx);
        // Publish the obtained context BEFORE the write (PR #1 review comment 4212512400): if this
        // first write's response is lost, the UI goes "uncertain" and Retry must resume the SAME
        // journalled request (same purchase context + Idempotency-Key). With context set only on
        // success the Retry button was dead and the buyer's only escape was a second purchase.
        adoptContext(ctx);
        if (known) {
          setOrderID(known);
          setProblem("order");
          return false;
        }
        const current = await readPurchase("cart", ctx, validCart);
        const result = await writePurchase(ctx, { kind: "cart", body: build(current) });
        if (result.kind !== "cart") throw new Error("unexpected result");
        setCart(result.value);
        setOrderID(null);
        return true;
      } catch (reason) {
        const kind = cartProblem(reason);
        setProblem(kind);
        if (kind === "conflict" || kind === "session") {
          working.current = false;
          void refresh();
        }
        return false;
      } finally {
        working.current = false;
        setBusy(false);
      }
    },
    [refresh, adoptContext],
  );

  const add = useCallback(
    (sku: string, quantity: number) =>
      mutate((current) => cartWithQuantity(current, sku, (current.items.find((i) => i.sku_id === sku)?.quantity ?? 0) + quantity)),
    [mutate],
  );
  const setQuantity = useCallback((sku: string, quantity: number) => mutate((current) => cartWithQuantity(current, sku, quantity)), [mutate]);

  // "uncertain": resume the SAME journalled request (same idempotency key) instead of composing a new one.
  const retry = useCallback(async () => {
    if (working.current || !context) return;
    working.current = true;
    setBusy(true);
    try {
      const result = await writePurchase(context);
      if (result.kind === "cart") setCart(result.value);
      setProblem(null);
    } catch (reason) {
      setProblem(cartProblem(reason));
    } finally {
      working.current = false;
      setBusy(false);
      void refresh();
    }
  }, [context, refresh]);

  const startNewCart = useCallback(async () => {
    if (working.current || !context || !orderID) return;
    working.current = true;
    setBusy(true);
    try {
      setCart(await continueShopping(context, orderID));
      setOrderID(null);
      setProblem(null);
    } catch (reason) {
      setProblem(cartProblem(reason));
    } finally {
      working.current = false;
      setBusy(false);
    }
  }, [context, orderID]);

  const value = useMemo<CartApi>(
    () => ({
      cart,
      context,
      count: cartCount(cart),
      ready,
      busy,
      problem,
      orderID,
      drawerOpen,
      setDrawerOpen,
      add,
      setQuantity,
      refresh,
      retry,
      startNewCart,
      dismiss: () => setProblem(null),
    }),
    [cart, context, ready, busy, problem, orderID, drawerOpen, add, setQuantity, refresh, retry, startNewCart],
  );
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
