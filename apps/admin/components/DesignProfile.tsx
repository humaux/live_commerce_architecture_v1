"use client";

// Store profile tab of the Design editor: name, tagline, logo, favicon, accent colour (native colour input), announcement
// and contact/social links. Data: the `profile` object of the design document (contracts/storefront-v2.md section B), saved
// by Design.tsx through PUT /api/stores/{store}/design/draft -> Go internal/httpapi/design.go. Pure form: no request here.
import type { DesignCopy } from "@/lib/design-copy";
import type { DesignDocument, MediaItem } from "@/lib/design-model";
import { ImagePicker, type MediaOps } from "./DesignMedia";
import { Field } from "./DesignField";

type Profile = DesignDocument["profile"];

export function DesignProfile({
  store, profile, media, ops, c, issue, onChange,
}: {
  store: string; profile: Profile; media: MediaItem[]; ops: MediaOps; c: DesignCopy; issue: (path: string) => string; onChange: (next: Profile) => void;
}) {
  const set = <K extends keyof Profile>(key: K, value: Profile[K]) => onChange({ ...profile, [key]: value });
  const contact = (key: keyof Profile["contact"], value: string) => onChange({ ...profile, contact: { ...profile.contact, [key]: value } });
  const p = c.profile;
  return (
    <div className="design-panel" data-testid="design-profile">
      <Field label={p.name} error={issue("profile.name")}>
        <input value={profile.name} maxLength={60} onChange={(e) => set("name", e.target.value)} data-testid="design-name" />
      </Field>
      <Field label={p.tagline}>
        <input value={profile.tagline ?? ""} maxLength={120} onChange={(e) => set("tagline", e.target.value)} />
      </Field>
      <div className="design-two">
        <ImagePicker store={store} label={p.logo} value={profile.logo_image_id} media={media} ops={ops} c={c} testId="design-logo" onChange={(id) => set("logo_image_id", id)} />
        <ImagePicker store={store} label={p.favicon} value={profile.favicon_image_id} media={media} ops={ops} c={c} testId="design-favicon" onChange={(id) => set("favicon_image_id", id)} />
      </div>
      <Field label={p.accent}>
        <span className="design-color">
          <input type="color" value={/^#[0-9a-fA-F]{6}$/.test(profile.accent_color) ? profile.accent_color : "#247965"} onChange={(e) => set("accent_color", e.target.value)} data-testid="design-accent" />
          <code>{profile.accent_color}</code>
        </span>
      </Field>
      <Field label={p.announcement}>
        <input value={profile.announcement ?? ""} maxLength={140} onChange={(e) => set("announcement", e.target.value)} />
      </Field>
      <fieldset className="design-fieldset">
        <legend>{p.contact}</legend>
        <div className="design-two">
          <Field label={p.email} error={issue("profile.contact.email")}><input type="email" inputMode="email" value={profile.contact.email ?? ""} maxLength={120} onChange={(e) => contact("email", e.target.value)} /></Field>
          <Field label={p.phone} error={issue("profile.contact.phone")}><input type="tel" value={profile.contact.phone ?? ""} maxLength={30} onChange={(e) => contact("phone", e.target.value)} /></Field>
        </div>
        <Field label={p.address}><input value={profile.contact.address ?? ""} maxLength={200} onChange={(e) => contact("address", e.target.value)} /></Field>
        <Field label={p.line} error={issue("profile.contact.line_url")}><input type="url" value={profile.contact.line_url ?? ""} onChange={(e) => contact("line_url", e.target.value)} /></Field>
        <div className="design-two">
          <Field label={p.facebook} error={issue("profile.contact.facebook_url")}><input type="url" value={profile.contact.facebook_url ?? ""} onChange={(e) => contact("facebook_url", e.target.value)} /></Field>
          <Field label={p.instagram} error={issue("profile.contact.instagram_url")}><input type="url" value={profile.contact.instagram_url ?? ""} onChange={(e) => contact("instagram_url", e.target.value)} /></Field>
        </div>
      </fieldset>
    </div>
  );
}
