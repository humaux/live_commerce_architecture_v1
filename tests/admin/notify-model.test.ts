// buyer-comms: strict parser + body of the new-order mail opt-out (apps/admin/lib/notify-model.ts), the BFF request grammar entry and copy parity of
// the three admin locales (lib/notify-copy.ts). Run: node --test --experimental-strip-types tests/admin/notify-model.test.ts
import assert from "node:assert/strict";
import { test } from "node:test";
import { notifySettingsBody, parseNotifySettings } from "../../apps/admin/lib/notify-model.ts";
import { notifyCopy } from "../../apps/admin/lib/notify-copy.ts";
import { logisticsRoute } from "../../apps/admin/lib/logistics-request.ts";

test("settings answer: exactly one boolean key", () => {
  assert.deepEqual(parseNotifySettings({ merchant_new_order_email: true }), { merchant_new_order_email: true });
  assert.equal(parseNotifySettings({ merchant_new_order_email: false }).merchant_new_order_email, false);
  for (const bad of [null, [], {}, { merchant_new_order_email: "yes" }, { merchant_new_order_email: null }, { merchant_new_order_email: true, x: 1 }, "x"])
    assert.throws(() => parseNotifySettings(bad), /unavailable/);
});

test("PUT body is the frozen single key", () => {
  assert.equal(notifySettingsBody(false), '{"merchant_new_order_email":false}');
  assert.equal(notifySettingsBody(true), '{"merchant_new_order_email":true}');
});

test("BFF grammar: GET read, PUT keyed command, nothing else", () => {
  assert.equal(logisticsRoute("GET", "notification-settings"), "get");
  assert.equal(logisticsRoute("PUT", "notification-settings"), "command");
  for (const [m, p] of [["POST", "notification-settings"], ["DELETE", "notification-settings"], ["GET", "notification-settings/x"], ["GET", "Notification-Settings"], ["PUT", "notification-settings?x=1"]])
    assert.equal(logisticsRoute(m, p), null, `${m} ${p}`);
});

test("copy: three locales, same keys, no empty text", () => {
  const keys = Object.keys(notifyCopy.en).sort();
  for (const locale of ["zh-CN", "zh-TW", "en"] as const) {
    assert.deepEqual(Object.keys(notifyCopy[locale]).sort(), keys, locale);
    for (const [k, v] of Object.entries(notifyCopy[locale])) assert.ok(v.length > 0, `${locale}.${k}`);
  }
});
