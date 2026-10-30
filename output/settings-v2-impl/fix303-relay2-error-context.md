# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: settings-real.spec.ts >> REAL_PG A wizard has no PAYUNi, creates unseeded configuration and preserves safe uncertainty
- Location: tests/admin/settings-real.spec.ts:28:1

# Error details

```
Error: expect(locator).toHaveValue(expected) failed

Locator: getByLabel('Product name', { exact: true })
Expected: "Acceptance draft bag"
Timeout: 10000ms
Error: element(s) not found

Call log:
  - Expect "toHaveValue" getByLabel('Product name', { exact: true }) with timeout 10000ms
  - waiting for getByLabel('Product name', { exact: true })

```

```yaml
- link "Skip to content":
  - /url: "#main"
- banner:
  - button "Open navigation"
  - text: Switch store
  - combobox "Switch store":
    - option "Settings acceptance store" [selected]
  - text: Language
  - combobox "Language":
    - option "简体中文"
    - option "繁體中文"
    - option "English" [selected]
  - group: Help
  - group: Account
- main:
  - navigation "Workspace navigation":
    - link "Overview":
      - /url: /en/
    - text: Settings Store settings
  - heading "Review settings" [level=1]
  - paragraph: Stripe payment eligibility pending; payment timing depends on the buyer’s method. No cash on delivery.
  - paragraph: Arrange shipping and record the carrier and tracking number. No automatic shipment creation or label purchase.
  - paragraph: Delivery availability is confirmed at checkout.
  - paragraph: "Store name: Settings acceptance store"
  - paragraph: "Email: settings@example.test"
  - paragraph: "Phone (optional): 000-000-1111"
  - paragraph: "Home delivery: off"
  - paragraph: "7-ELEVEN: off"
  - paragraph: "FamilyMart: off"
  - paragraph: "Hi-Life: off"
  - paragraph: "OK: off"
  - paragraph: "Product draft: Acceptance draft bag"
  - paragraph: "Price (TWD): 1280"
  - paragraph: "Stock (units): 10"
  - paragraph: Product is saved as a draft, not published.
  - button "Save settings"
  - button "Back to edit"
  - paragraph: DaWan Live is a live-commerce platform of Hong Kong Da Wan Trading Limited
- alert
```
