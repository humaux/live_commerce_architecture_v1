---
name: DaWan Live public platform
description: Owner-approved A surface; scoped to public routes, not merchant pages or W0.
colors:
  canvas: "#ffffff"
  ink: "#1f2937"
  action: "#ff6a00"
  focus: "#a84300"
  brand-text: "#c24c00"
  border: "#e5e7eb"
typography:
  body:
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "16px"
    lineHeight: 1.6
rounded:
  panel: "8px"
spacing:
  small: "8px"
  medium: "16px"
  large: "24px"
components:
  primary-link:
    backgroundColor: "{colors.action}"
    textColor: "{colors.ink}"
    rounded: "{rounded.panel}"
    padding: "10px 32px"
---

# Design System: DaWan Live public platform

## Overview

**Creative North Star: "From comments to orders"**

This public surface explains a product workflow, then lets visitors inspect the operator's policies and open the separate merchant application. It extends the approved A composition without replacing the root ledger/W0 design system. The brief and actual rendered public pages, not the generated comp's incidental data, define this boundary.

Key characteristics: white canvas, slate type, orange actions, flat workflow examples, complete operator attribution. Authority: `.impeccable/platform-site-brief.md` and owner A selection, decision key `880ef0da` (not a FORM roll seed).

## Colors

Action orange marks registration and login; darker orange identifies the wordmark accent and current language. Slate is the text and footer background. Fine gray borders separate header, workflow previews and facts. Palette values are defined above and evidenced by `apps/admin/components/platform/platform.css`.

**The Contrast Rule.** Orange primary links use dark text; footer focus indicators use a lighter outline against slate.

## Typography

Body and controls retain the approved system-sans family. The current hero uses heavy system type, a 40–64px responsive scale and tight leading; this is an A-surface implementation detail, not a new display-font rule for future product pages. Legal copy uses a readable single column with clear section headings.

## Layout

Public content has a 1440px maximum width. The desktop hero has a larger headline column beside a smaller benefit column; three workflow panels follow. At 1000px the composition tightens and inter-panel connectors disappear. At 680px the hero and panels stack, the header wraps, and horizontal gutters become 20px. Legal documents have a 900px maximum width.

**The Identity Placement Rule.** The home footer starts below the first viewport. Every public footer and every legal/contact body retains full operator facts; this does not apply to the merchant storefront.

## Elevation & Depth

Borders and subtle neutral preview surfaces establish hierarchy; there is no ambient shadow system. The inset current-language underline expresses navigation state, not elevation. The garment's photographic shadow belongs only to the synthetic raster.

## Shapes

Repeated panels and primary actions use gently rounded corners. Workflow step numbers and synthetic avatars are circular. These shapes do not imply that the static preview panels are interactive.

## Components

- **Navigation:** the brand returns home, language links preserve the public document, and login points to the configured admin host. Targets are at least 44px; focus is visible. Skip-to-content targets the focusable main element.
- **Primary link:** an orange, dark-text anchor with rounded corners; registration is the only hero call to action. Browser-native link semantics remain intact.
- **Workflow previews:** three labelled static examples, fine borders and a nearby setup disclaimer. The generated garment has a prompt sidecar and is never represented as a real merchant asset.
- **Company facts/footer:** shared source in `company.ts`; descriptive labels localize, legal facts do not. The dark footer repeats public policy links and full company facts.
- **Legal article:** single-column section headings and prose; no input, form, notification or dashboard component is introduced.

## Do's and Don'ts

- **Do** preserve locale choice, visible keyboard focus and 44px touch targets.
- **Do** keep diagram examples separate from connection status and commercial claims.
- **Do** read hosts and public contact details from configuration.
- **Don't** transfer this marketing composition into the merchant storefront or W0 shell.
- **Don't** treat demo names, prices, status labels or the garment as reusable product evidence.

Not canonized: the current heavy system display face and arrow glyphs are retained A-surface details, not new reusable craft rules. No separate quality-bar card was available for this unit. Source-based documentation is not a claim of legal approval, production deployment or live Meta verification.
