"use client";

// Purpose: gives client components the shop display name from the server-rendered design document
// (no client-side store fetch exists). Consumed by the platform collector disclosure in OrderPayment
// (stripe-platform-account-v1 §5: "由 {display_name} 代 {store_name} 收取").
// Depends on: react context only.
// Used by: app/[locale]/layout.tsx (provider), components/OrderPayment.tsx (consumer).
import { createContext, useContext, type ReactNode } from "react";

const ShopNameContext = createContext("");

export function ShopNameProvider({ name, children }: { name: string; children: ReactNode }) {
  return <ShopNameContext.Provider value={name}>{children}</ShopNameContext.Provider>;
}

export function useShopName(): string {
  return useContext(ShopNameContext);
}
