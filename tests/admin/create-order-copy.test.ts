// Purpose: every server order refusal and live/send reason has safe localized buyer-order guidance.
// Depends on: real create-order copy/model and manual copy; no component or BFF mocks.
// Used by: LC-U3 Node gate and fail-closed price copy regression.
import test from "node:test";
import assert from "node:assert/strict";
import {
  createOrderCopy,
  createOrderReason,
  createOrderError,
} from "../../apps/admin/lib/create-order-copy.ts";
import {
  forBuyerErrors,
  livePriceReasons,
  forBuyerSendReasons,
} from "../../apps/admin/lib/create-order-model.ts";
import { toolsCopy } from "../../apps/admin/lib/merchant-tools-copy.ts";
for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
  test(`LC-U3 all coded errors have explicit safe copy ${locale}`, () => {
    for (const codes of Object.values(forBuyerErrors))
      for (const code of codes) {
        const text = createOrderError(locale, code);
        assert.notEqual(text, toolsCopy[locale].manual.errors.default, code);
        assert.notEqual(text, code);
      }
    assert.equal(
      createOrderError(locale, "<script>private</script>"),
      toolsCopy[locale].manual.errors.default,
    );
  });
  test(`LC-U3 eligibility and every live/send reason ${locale}`, () => {
    assert.match(
      createOrderCopy[locale].liveWarning,
      locale === "en"
        ? /Live pricing does not apply/
        : /直播价不适用|直播價不適用/,
    );
    for (const code of [...livePriceReasons, ...forBuyerSendReasons].filter(
      (x) => x !== "" && x !== "unknown",
    ))
      assert.notEqual(
        createOrderReason(locale, code),
        createOrderCopy[locale].unknown,
        code,
      );
    assert.equal(
      createOrderReason(locale, "private_canary"),
      createOrderCopy[locale].unknown,
    );
  });
}
