// Platform operator attribution only; no BFF/Go calls and no merchant-storefront branding.
import { operatedBy } from "@/lib/company";

export function OperatorFooter() {
  return (
    <footer className="operator-footer" data-testid="operator-footer">
      <p>{operatedBy}</p>
    </footer>
  );
}
