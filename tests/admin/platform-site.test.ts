// PS1/PS4: platform identity/configuration and host routing; no provider calls.
import { test } from "node:test";
import assert from "node:assert/strict";
import { company, companyConfig, operatedBy, platformRoute } from "../../apps/admin/lib/company.ts";

const env = { NODE_ENV: "production", LC_PLATFORM_HOST: "platform.example.invalid", LC_ADMIN_HOST: "admin.example.invalid", LC_COMPANY_CONTACT_EMAIL: "contact@example.invalid" };
test("PS1 legal identity is one untranslated source", () => {
  assert.equal(company.productName, "DaWan Live");
  assert.equal(company.legalEnglish, "Hong Kong Da Wan Trading Limited");
  assert.equal(company.legalChinese, "香港大碗貿易有限公司");
  assert.equal(company.registrationNumber, "81215167");
  assert.equal(company.businessRegistrationNumber, "81215167-000-09-26-2");
  assert.equal(company.incorporatedOn, "2026-09-11");
  assert.equal(company.address, "RM 10, 23/F, New Trend Centre, 704 Prince Edward Road East, San Po Kong, Hong Kong");
  assert.equal(operatedBy, `${company.productName} is operated by ${company.legalEnglish}`);
});
test("PS1 contact and hosts come only from configuration; production fails closed", () => {
  assert.equal(companyConfig(env).email, env.LC_COMPANY_CONTACT_EMAIL);
  for (const field of ["LC_COMPANY_CONTACT_EMAIL", "LC_PLATFORM_HOST", "LC_ADMIN_HOST"] as const) {
    assert.throws(() => companyConfig({ ...env, [field]: "" }), new RegExp(field));
  }
  for (const value of ["bad", "<a>@example.invalid", "a@example.invalid\r\nBcc:x", ".a@example.invalid", "a..b@example.invalid", "a@example..invalid", "a@-example.invalid"]) {
    assert.throws(() => companyConfig({ ...env, LC_COMPANY_CONTACT_EMAIL: value }));
  }
  for (const host of ["https://evil.invalid", "host.invalid/path", "host.invalid:443", "evil.invalid@ok.invalid"]) {
    assert.throws(() => companyConfig({ ...env, LC_PLATFORM_HOST: host }));
  }
  assert.throws(() => companyConfig({ ...env, LC_ADMIN_HOST: env.LC_PLATFORM_HOST }));
});
test("PS2 public routes default to zh-TW, restrict page/locale, and never include admin", () => {
  assert.deepEqual(platformRoute("/"), { locale: "zh-TW", page: "home" });
  for (const locale of ["zh-TW", "zh-CN", "en"]) {
    for (const page of ["privacy", "terms", "data-deletion", "contact"]) {
      assert.deepEqual(platformRoute(`/${locale}/${page}`), { locale, page });
    }
  }
  for (const path of ["/api/auth/login", "/en/orders", "/platform/en", "/en/privacy/extra", "/fr/contact", "/%65n/contact"]) assert.equal(platformRoute(path), null);
});
test("PS4 optional Meta verification value never has a fake default", () => {
  assert.equal(companyConfig(env).verification, undefined);
  assert.equal(companyConfig({ ...env, LC_META_DOMAIN_VERIFICATION: "fixture-domain-proof" }).verification, "fixture-domain-proof");
});
