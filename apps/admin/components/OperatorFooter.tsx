// Platform operator attribution only; no BFF/Go calls and no merchant-storefront branding.
import { operatorSentence, type PlatformLocale } from "@/lib/company";

export function OperatorFooter({ locale }: { locale: PlatformLocale }) {
  return (
    <footer className="operator-footer" data-testid="operator-footer">
      <p>{operatorSentence(locale)}</p>
    </footer>
  );
}
