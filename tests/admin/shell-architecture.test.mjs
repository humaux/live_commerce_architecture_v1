import { test } from "node:test";
import assert from "node:assert/strict";
import {
  architectureGate,
  findings,
  validateAllowances,
  imports,
  deepFeatureImport,
} from "../../scripts/dev/ui-architecture-gate.mjs";
test("G-UI3/G-UI5 architecture constraints pass with shrinking legacy exceptions", () =>
  assert.deepEqual(architectureGate().errors, []));
test("G-UI3/G-UI5 negatives: new formatter, component fetch, >800 lines and wider allowance fail", () => {
  assert.deepEqual(
    findings(
      "apps/admin/src/components/Bad.tsx",
      'new Intl.NumberFormat("en"); fetch("/api/x")',
    ),
    { format: 1, fetch: 1 },
  );
  assert.equal(
    findings("packages/ui/src/Bad.tsx", "\n".repeat(801)).lines,
    802,
  );
  assert.ok(validateAllowances({ "new.ts": { fetch: 1 } }, {}).length);
  assert.ok(
    validateAllowances({ "old.ts": { fetch: 2 } }, { "old.ts": { fetch: 1 } })
      .length,
  );
  assert.deepEqual(validateAllowances({}, { "old.ts": { fetch: 1 } }), []);
  assert.equal(
    findings("apps/admin/src/Bad.tsx", "<p>最小貨幣單位</p>").minorCopy,
    1,
  );
  assert.equal(
    findings("apps/admin/src/Bad.tsx", 'const label = "minor units"').minorCopy,
    1,
  );
  assert.equal(
    findings(
      "apps/admin/src/Good.tsx",
      '// minor units\nconst field = "price_minor"',
    ).minorCopy,
    undefined,
  );
  const source = 'export { catalogRoutes } from "../catalog/routes"';
  const targets = imports("apps/admin/src/features/orders/routes.ts", source);
  assert.equal(
    targets.length,
    1,
    "compiler resolves relative cross-feature re-export",
  );
  assert.ok(
    deepFeatureImport("apps/admin/src/features/orders/routes.ts", targets[0]),
  );
  assert.equal(
    deepFeatureImport(
      "apps/admin/src/features/orders/routes.ts",
      "/src/features/catalog/index.ts",
    ),
    false,
  );
});
