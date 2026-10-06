// Purpose: Checks company configuration, public host routing and localized disclosures.
// Depends on: node:test, node:assert/strict, ../../apps/admin/lib/company.ts, ../../apps/admin/lib/platform-copy.ts, ../../apps/admin/lib/platform-legal.ts, node:fs, ../../apps/admin/lib/company.ts, ../../apps/admin/lib/meta-connect-copy.ts, ./platform-messaging-copy.fixture.mjs
// Used by: scripts/dev/test-local.sh, scripts/dev/test-node.sh
// PS1/PS4: platform identity/configuration and host routing; no provider calls.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  company,
  companyConfig,
  operatedBy,
  platformRoute,
  platformPath,
  platformLocales,
  platformPages,
  brandedTitle,
} from "../../apps/admin/lib/company.ts";
import { platformCopy } from "../../apps/admin/lib/platform-copy.ts";
import { platformLegal } from "../../apps/admin/lib/platform-legal.ts";
import { readFileSync } from "node:fs";
import * as platformCompany from "../../apps/admin/lib/company.ts";
import { metaConnectCopy } from "../../apps/admin/lib/meta-connect-copy.ts";
import { assertPlatformMessagingCopy } from "./platform-messaging-copy.fixture.mjs";

const env = {
  NODE_ENV: "production",
  LC_PLATFORM_HOST: "platform.example.invalid",
  LC_ADMIN_HOST: "admin.example.invalid",
  LC_COMPANY_CONTACT_EMAIL: "contact@example.invalid",
};
test("ADM10 operator sentence is localized while product and company facts stay authoritative", () => {
  assert.equal(typeof platformCompany.operatorSentence, "function");
  assert.equal(
    platformCompany.operatorSentence("zh-TW"),
    `${company.productName} 由 ${company.legalChinese} 營運`,
  );
  assert.equal(
    platformCompany.operatorSentence("zh-CN"),
    `${company.productName} 由 ${company.legalChinese} 运营`,
  );
  assert.equal(platformCompany.operatorSentence("en"), operatedBy);
});
test("PS review: actual Host matching normalizes case, port and a terminal DNS dot, never forwarded authority", () => {
  assert.equal(typeof platformCompany.requestHostname, "function");
  for (const value of [
    "PLATFORM.Example.Invalid",
    "platform.example.invalid.",
    "PLATFORM.EXAMPLE.INVALID.:443",
  ])
    assert.equal(
      platformCompany.requestHostname(value),
      "platform.example.invalid",
    );
  for (const value of [
    null,
    "",
    "https://platform.example.invalid",
    "platform.example.invalid@evil.invalid",
    "platform.example.invalid:invalid",
    "platform.example.invalid:99999",
    "platform.example.invalid..",
    "platform.example.invalid,evil.invalid",
  ])
    assert.equal(platformCompany.requestHostname(value), null);
});
test("PS review: privacy/terms retain Messenger and Instagram purpose disclosure; deletion uses exact UI labels", () => {
  for (const locale of platformLocales) {
    for (const page of ["privacy", "terms"] as const) {
      const copy = platformLegal[locale][page]
        .map(([, text]) => text)
        .join(" ");
      assert.ok(copy.includes("Messenger"), `${locale}/${page}: Messenger`);
      assert.ok(copy.includes("Instagram"), `${locale}/${page}: Instagram`);
    }
    const deletion = platformLegal[locale]["data-deletion"][0][1];
    assert.ok(
      deletion.includes(metaConnectCopy[locale].title),
      `${locale}: exact settings card`,
    );
    assert.ok(
      deletion.includes(metaConnectCopy[locale].disconnect),
      `${locale}: exact disconnect button`,
    );
  }
});
// pages_messaging/instagram_manage_messages purposes stay disclosed, without
// pretending inbound DMs are subscribed or that a send result proves delivery.
for (const locale of platformLocales) {
  for (const page of ["privacy", "terms"] as const) {
    test(`P2-N1 ${locale}/${page}: current comments and private reply, conditional inbound messages`, () => {
      const text = platformLegal[locale][page]
        .map(([, body]) => body)
        .join(" ");
      assertPlatformMessagingCopy(text, locale, page);
    });
  }
}
test("PS1 legal identity is one untranslated source", () => {
  assert.equal(company.productName, "DaWan Live");
  assert.equal(company.legalEnglish, "Hong Kong Da Wan Trading Limited");
  assert.equal(company.legalChinese, "香港大碗貿易有限公司");
  assert.equal(company.registrationNumber, "81215167");
  assert.equal(company.businessRegistrationNumber, "81215167-000-09-26-2");
  assert.equal(company.incorporatedOn, "2026-09-11");
  assert.equal(company.businessRegistrationValidity, "11/09/2026 – 10/09/2027");
  assert.equal(
    company.address,
    "RM 10, 23/F, New Trend Centre, 704 Prince Edward Road East, San Po Kong, Hong Kong",
  );
  assert.equal(
    operatedBy,
    `${company.productName} is operated by ${company.legalEnglish}`,
  );
});
test("PS1 contact and hosts come only from configuration; production fails closed", () => {
  assert.equal(companyConfig(env).email, env.LC_COMPANY_CONTACT_EMAIL);
  for (const field of [
    "LC_COMPANY_CONTACT_EMAIL",
    "LC_PLATFORM_HOST",
    "LC_ADMIN_HOST",
  ] as const) {
    assert.throws(
      () => companyConfig({ ...env, [field]: "" }),
      new RegExp(field),
    );
  }
  for (const value of [
    "bad",
    "<a>@example.invalid",
    "a@example.invalid\r\nBcc:x",
    ".a@example.invalid",
    "a..b@example.invalid",
    "a@example..invalid",
    "a@-example.invalid",
  ]) {
    assert.throws(() =>
      companyConfig({ ...env, LC_COMPANY_CONTACT_EMAIL: value }),
    );
  }
  for (const host of [
    "https://evil.invalid",
    "host.invalid/path",
    "host.invalid:443",
    "evil.invalid@ok.invalid",
  ]) {
    assert.throws(() => companyConfig({ ...env, LC_PLATFORM_HOST: host }));
  }
  assert.throws(() =>
    companyConfig({ ...env, LC_ADMIN_HOST: env.LC_PLATFORM_HOST }),
  );
});
test("PS2 public routes default to zh-TW, restrict page/locale, and never include admin", () => {
  assert.deepEqual(platformRoute("/"), { locale: "zh-TW", page: "home" });
  for (const locale of ["zh-TW", "zh-CN", "en"]) {
    for (const page of ["privacy", "terms", "data-deletion", "contact"]) {
      assert.deepEqual(platformRoute(`/${locale}/${page}`), { locale, page });
    }
  }
  for (const path of [
    "/api/auth/login",
    "/en/orders",
    "/platform/en",
    "/en/privacy/extra",
    "/fr/contact",
    "/%65n/contact",
  ])
    assert.equal(platformRoute(path), null);
});
test("PS4 optional Meta verification value never has a fake default", () => {
  assert.equal(companyConfig(env).verification, undefined);
  assert.equal(
    companyConfig({
      ...env,
      LC_META_DOMAIN_VERIFICATION: "fixture-domain-proof",
    }).verification,
    "fixture-domain-proof",
  );
});
test("PS1 every locale uses the same product name in auth page titles", () => {
  for (const title of [
    "登入",
    "登录",
    "Sign in",
    "建立帳號",
    "创建账号",
    "Create account",
    "重設密碼",
    "重设密码",
    "Reset password",
  ]) {
    assert.equal(brandedTitle(title), `DaWan Live · ${title}`);
  }
});
test("PS1 public page copy has three complete locales, no invented operator identity or contact", () => {
  for (const locale of platformLocales) {
    assert.deepEqual(
      Object.keys(platformCopy[locale].pages).sort(),
      [...platformPages].sort(),
    );
    assert.deepEqual(
      Object.keys(platformLegal[locale]).sort(),
      platformPages.filter((p) => p !== "home").sort(),
    );
    for (const page of platformPages) {
      assert.deepEqual(platformRoute(platformPath(locale, page)), {
        locale,
        page,
      });
      if (page !== "home")
        assert.ok(
          platformLegal[locale][page].every(
            ([title, text]) => title && text.length > 20,
          ),
        );
    }
    assert.equal(platformCopy[locale].steps.length, 3);
    assert.ok(platformCopy[locale].disclaimer.length > 40);
    const copy = JSON.stringify([platformCopy[locale], platformLegal[locale]]);
    assert.ok(!copy.includes("@"));
    assert.ok(
      !copy.includes(company.legalEnglish),
      "operator identity must be rendered by the single CompanyFacts component",
    );
  }
});
test("PS1 production build validates config; public data is not bundled through NEXT_PUBLIC", () => {
  const source = readFileSync("apps/admin/next.config.ts", "utf8");
  assert.match(source, /phase === PHASE_PRODUCTION_BUILD\) companyConfig\(\)/);
  assert.ok(!source.includes("env: {"));
  for (const name of ["PlatformDocument.tsx", "PlatformHome.tsx"]) {
    const component = readFileSync(
      `apps/admin/components/platform/${name}`,
      "utf8",
    );
    assert.ok(!component.includes("dangerouslySetInnerHTML"));
  }
});
test("PS review: admin image installs the manifests for its workspace dependencies before building", () => {
  const dockerfile = readFileSync("deploy/docker/admin.Dockerfile", "utf8");
  const install = dockerfile.indexOf("pnpm install --frozen-lockfile");
  assert.ok(install > 0);
  const manifest = JSON.parse(readFileSync("apps/admin/package.json", "utf8"));
  for (const [name, version] of Object.entries(manifest.dependencies)) {
    if (version !== "workspace:*") continue;
    const folder = name.replace("@live-commerce/", "");
    const copy = `COPY packages/${folder}/package.json packages/${folder}/`;
    const position = dockerfile.indexOf(copy);
    assert.ok(position >= 0 && position < install, copy);
  }
});
