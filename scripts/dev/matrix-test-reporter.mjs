// Purpose: stream native Node test-runner events for execution receipts.
// Depends on: Node's custom test reporter async-iterable contract.
// Used by: matrix-receipt.mjs; event semantics are validated from actual execution.
export default async function* matrixReporter(source) {
  for await (const event of source) yield `${JSON.stringify(event, (_key, value) => value instanceof Error
    ? { ...value, name: value.name, message: value.message, stack: value.stack } : value)}\n`;
}
