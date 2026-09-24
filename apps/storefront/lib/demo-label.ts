// Visual disclosure only, never an auth/billing bypass. Public remote API
// origins fail closed even if this local fixture flag is accidentally set.
export function buyerDemoLabel(
  env: Record<string, string | undefined>,
): boolean {
  return (
    env.COMMERCE_BUYER_DEMO_LABEL === "1" &&
    /^http:\/\/127\.0\.0\.1:\d+$/.test(env.COMMERCE_BUYER_API_ORIGIN ?? "")
  );
}
