# Design direction: 商品对账簿

Direction selected by the user on 2026-09-20 in decision `6ecd43a2`: assigned/A. This commits the visual world, not yet the final composition. Build path is comp-first.

- Mode: operate. A merchant can identify a SKU, inspect the on-hand/reserved/available relationship, and act with an auditable reason.
- Palette: navy navigation `#193c61`, cool neutral canvas `#f5f7fa`, white working surfaces, restrained teal actions `#2b7666`; semantic states always include words, not color alone.
- Type: contemporary readable Chinese sans-serif with Latin system fallback. Numeric columns use tabular figures; restrained hierarchy, no display lettering.
- Material: clean ledger paper translated into a digital table, thin cool rules, lightly rounded controls, quiet separation, minimal elevation.
- Signature: the SKU ledger and its contextual inspector are the focal relationship. Real product thumbnails belong to product data; demo assets must be visibly labeled and never shipped as customer inventory.
- Motion: short state transitions and explicit pending/success/error feedback, respecting reduced motion. No decorative loading loops.
- Domain separation: website customer service, Meta messages, and platform support retain separate navigation and authorization scopes.
- Language: zh-CN, zh-TW, en are peers. Language selection never changes store, permissions, currency, market, or cart.
- Truth boundary: generated mock copy and sample dates are not specifications. No unsupported channel synchronization promise, fabricated alerts, automatic sales claims, or working-looking controls without an implemented action.

Selected direction image: `.impeccable/mocks/decision/assigned.png`. Final composition approval remains pending; record it in the surface brief before UI implementation.
