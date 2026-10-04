// Labelled form field of the Design editor: label text, control, and the inline error line (role=alert) used for both
// local "required" hints and the server's 422 path message. Presentational only.
import type { ReactNode } from "react";

export function Field({ label, error, children }: { label: string; error?: string; children: ReactNode }) {
  return (
    <label className={error ? "design-field has-error" : "design-field"}>
      <span className="design-label">{label}</span>
      {children}
      {error ? <small className="design-error" role="alert">{error}</small> : null}
    </label>
  );
}
