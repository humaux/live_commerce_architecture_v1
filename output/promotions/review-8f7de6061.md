**结论：APPROVE**

PR #82 · head 8f7de6061b17c9eb4a8fecb11e124da4e1ad6773 · base r3/integration · 独立复核（只读，未跑测试；CI 80 pass / 1 skip 由调用方提供）
范围：`tests/storefront/amount-match.mjs`、`tests/storefront/amount-match.test.mjs` 的 diff，以及直接调用方 `tests/storefront/promotions-gate.mjs`（`amt()` 调用点 183–299）。

## 检查项

1. **对真实金额的严格度没有放宽**：通过
   - 新正则 `(?<![0-9A-Za-z,.])(?<!BRP-)960(\.00)?(?![0-9,])` 和旧正则只差一个 `(?<!BRP-)`。它只在数字前面正好是字面 `BRP-` 时拒绝匹配，大小写敏感。
   - `NT$960` / `NT$ 960` / `960` / `960.00` / `合計NT$960` / `-960` 前面都不是 `BRP-`，仍能命中。已有用例 amount-match.test.mjs:8-12 覆盖这些写法，本 PR 在 :28 加了 `Discount -960`。
   - `BRP-` 就是 gate 自己造的 SKU 编码前缀：`tests/foundation/browser_promotions_test.go:64` 用 `Code: "BRP-" + tag`，`t04Tag` 是 12 位 hex（`tests/foundation/catalog_inventory_test.go:74`）。没有用宽泛的正则去剥离文本，真实金额前面不会出现 `BRP-`。
   - 这个 lookbehind 也作用于正向断言（promotions-gate.mjs:183/193/206/224-299）。正向断言因此只会更严，不会放过错误。
2. **RED→GREEN 证据**：通过
   - RED：f530a5504，CI run 39224605091（Foundation checks，conclusion=failure）。日志里 `✖ negative checks ignore fixture ids that start with the amount (review-93b987562 P2-1)`。当时 `withoutFixtureIds` 是恒等函数，等于旧行为：`doesNotMatch(refused, amt(96000))` 因 `BRP-960abcdef012` 失败。
   - GREEN：8f7de6061 是同一个注入串，测试见 amount-match.test.mjs:23-30。同时断言真实 `NT$960`（:27）和 `-960`（:28）仍命中，`NT$1,060` 不被误伤（:26）。CI 全绿。
   - 中间有一次尝试：d1fb6ee08 改写 gate:207，run 39225538032 失败（GATE_BASELINE 逐字守卫），之后被 8f7de6061 取代。
3. **没有改写 fixture，也没有删断言**：通过。只改了两个文件；promotions-gate.mjs、fixture、Go 测试都没动。测试文件只增不减（+9/-0）。
4. **文件头**：通过。amount-match.mjs:1-3 和 amount-match.test.mjs:1-3 都有 Purpose / Depends on / Used by。amount-match.mjs 的 Invariant 注释已改写，删掉了旧的 `ponytail:` 遗留说明（就是本 PR 修掉的那个问题）。

## P0
无。

## P1
无。

## P2
- **P2-1** PR 标题 / d1fb6ee08 的提交信息写的是 "promotions gate strips fixture ids"，但最终实现是 matcher 拒绝 `BRP-` 前缀，没有剥离 ID，gate 也没改。squash 合并时请改用 8f7de6061 的提交信息（"refuse the BRP- fixture prefix in the amount matcher, keep gate assertions verbatim"），免得历史被误读。（PR #82 title）
- **P2-2** tests/e2e/live-tools.spec.ts:52 的注释写着 "Same rule as tests/storefront/amount-match.mjs before the number"。amount-match 现在多了 `BRP-` 规则，两边已经不完全相同。这段注释只描述字母和数字的规则，仍然成立，不影响行为，可以下次顺手加一句。
- **P2-3**（已知上限，不需要改）：只防住 `BRP-` 这一个前缀。以后如果 gate 引入别的 `<PREFIX>-<hex>` 页面可见 ID，需要同样登记。amount-match.mjs:8-10 的注释已经写明了这一点。
