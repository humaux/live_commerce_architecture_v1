**结论：APPROVE**

# PR #69（DS-2）P1-A 复核 · head ffc699c7b（相对 2a2fc6431 的增量）

- 复核方式：只读，没有运行测试（NOT_RUN）。只看了 `git diff 2a2fc6431..ffc699c7b`，涉及 2 个文件（+10/-6）。
- 证据级：E2（读代码 + 对照账本）。
- 证据来源：
  - run 39256563874 的 gallery job 118075605740 结论是 success，headSha 是 ffc699c7b。整个 run 的结论在我查的时候还是空的，其他 job 可能还在跑。
  - 账本 `output/ds-2/gallery-39256563874/playwright/run.az5CnDQD/ui-gallery/click-ledger.json` 共 1132 行，0 失败。其中 wheel reach 67 行全部 pass，包括自检那一行。

## P1-A：已修

- **内滚动容器的判定改对了**（tests/admin/ui-gallery-clicks.mjs:707）：现在只有 `overflowX` 是 `auto` 或 `scroll`，并且内容确实溢出，才算内滚动容器。
- **`hidden` / `clip` 的祖先不再豁免**：这类容器裁掉的控件，按完整控件框取 elementFromPoint 的点。被裁掉那一侧的点会命中控件后面的元素，所以会判 out。
- **sideways 只在表格滚动容器里成立**：只有当所有内滚动祖先都带 `[data-table-scroll]` 时，才可能判 sideways。这个属性只出现在 `packages/ui/src/ds/Data.tsx:303` 的 DataTable 滚动容器上，也就是上轮建议的白名单（P2-A），这次一并做了。
- **表格外的局部裁切也会 fail**（:715）：不在表格里的内滚动容器，只要控件被横向裁掉一部分，就直接判 out。
- **账本对得上**：390 宽度下 sideways 仍然只有 components 每个语言 10 个、list 每个语言 5 个；自检仍是 12/12 被挡。规则收紧后，没有冒出新的失败。

## 其他改动

- 横向走查的 W3 TODO 已写在 :686–687，和上轮裁定的「之后再做」一致。
- 箭头的 `non-scaling-stroke` 已撤回（shell2.module.css）。撤回后渲染结果和以前一样，P2-B 关闭。

## 仍未修的（不阻塞）

- P2-C：自检只用了全屏遮罩，没加「只盖住一部分」的遮罩。
- P2-D：section 页签换成两行后，只有顶部是圆角。
