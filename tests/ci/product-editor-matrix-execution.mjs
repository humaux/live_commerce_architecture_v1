// Purpose: Bind matrix source/evidence gates to active Node and foundation CI commands, independently of receipt hashes.
// Depends on: Node fs/url, the existing declarative mode parser and foundation.yml job/step source; no shell evaluation.
// Used by: check-gates.sh and matrix execution corruption tests; shared harness hashes remain informational.
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { modeEntries } from "../../scripts/dev/pr-modes.mjs";

// Parse literal shell words without evaluation; the known dynamic planner
// expansions are removed before inspecting Go's additional argv.
function commandArgv(command) {
  const argv = [];
  let word = "", quote = null;
  for (let index = 0; index < command.length; index++) {
    const char = command[index];
    if (char === "\\" && quote !== "'") {
      if (index + 1 === command.length) return null;
      if (quote === '"' && !['"', "\\", "$", "`"].includes(command[index + 1])) word += char;
      else word += command[++index];
    } else if (quote) {
      if (char === quote) quote = null;
      else word += char;
    } else if (char === "'" || char === '"') quote = char;
    else if (/[;&|<>]/.test(char)) return null;
    else if (/\s/.test(char)) {
      if (word) argv.push(word);
      word = "";
    } else word += char;
  }
  if (quote) return null;
  if (word) argv.push(word);
  return argv;
}

// Assignment prefixes are shell metadata, not executable argv. Keep their values
// literal: command substitutions and environment references are never evaluated.
function executableArgv(command) {
  const argv = commandArgv(command);
  while (argv?.length && /^[A-Za-z_][A-Za-z0-9_]*=/.test(argv[0])) argv.shift();
  return argv;
}

function invokesActualGoTest(argv) {
  if (!argv) return false;
  if (argv[0] === "go") return argv[1] === "test";
  if (argv[0] !== "node" || argv[1] !== "scripts/dev/matrix-receipt.mjs" || argv[2] !== "go") return false;
  let index = 3;
  while (/^[A-Za-z_][A-Za-z0-9_]*=/.test(argv[index] || "")) index++;
  return argv[index] === "go" && argv[index + 1] === "test";
}

const goSelectionFlags = new Set(["-run", "-skip", "--run", "--skip", "-test.run", "-test.skip"]);
function hasGoSelectionFlags(args) {
  return args.some((arg) => goSelectionFlags.has(arg.split("=", 1)[0]));
}

function literalGoFlags(value) {
  const scalar = commandArgv(value);
  if (scalar === null) return null;
  const flags = [];
  for (const part of scalar) {
    const words = commandArgv(part);
    if (words === null) return null;
    flags.push(...words);
  }
  return flags;
}

function hasSelectingGoFlags(argv) {
  return argv.some((arg) => {
    const match = /^GOFLAGS=(.*)$/.exec(arg);
    if (!match) return false;
    const flags = literalGoFlags(match[1]);
    return flags === null || hasGoSelectionFlags(flags);
  });
}

function workflowHasSelectingGoFlags(text) {
  return text.split(/\r?\n/).some((line) => {
    const match = /^\s+GOFLAGS:\s*(.*?)\s*(?:#.*)?$/.exec(line);
    if (!match) return false;
    const flags = literalGoFlags(match[1]);
    return flags === null || hasGoSelectionFlags(flags);
  });
}

// Fail closed on the repository's small, indentation-based workflow subset.
// A filename in a name/env/comment or a disabled/unreachable run is not a binding;
// changed YAML/shell structure needs an explicit reviewed binding update.
function foundationRunnerBound(text, runner, condition = null) {
  const jobs = text.split(/^jobs:\s*$/m);
  if (jobs.length !== 2) return false;
  const blocks = [...jobs[1].matchAll(/^  foundation:\s*\n([\s\S]*?)(?=^  [^ #\n]|^\S|(?![\s\S]))/gm)];
  if (blocks.length !== 1) return false;
  const job = blocks[0][1];
  const continueOnError = [...job.matchAll(/^[ \t]+continue-on-error:[ \t]*(.*?)[ \t]*(?:#.*)?$/gim)];
  if (continueOnError.some(([, value]) => value !== "false")) return false;
  const steps = job.split(/^    steps:\s*$/m);
  if (steps.length !== 2 || /^    if:/m.test(job)) return false;
  return steps[1].split(/^      - /m).slice(1).some((step) => {
    const conditions = [...step.matchAll(/^(?:if:|        if:)\s*(.*)$/gm)];
    if (conditions.length > 1 || (conditions.length && conditions[0][1] !== condition)) return false;
    const runs = [...step.matchAll(/^(?:run:|        run:)\s*(.*)$/gm)];
    if (runs.length !== 1) return false;
    let command = runs[0][1];
    if (command === "|" || command === "|-") {
      const after = step.slice(runs[0].index + runs[0][0].length).split("\n");
      const body = [];
      for (const line of after) {
        if (line.trim() && !line.startsWith("          ")) break;
        if (line.trim() && !line.trim().startsWith("#")) body.push(line.trim());
      }
      if (body.length !== 1) return false;
      command = body[0];
    }
    const argv = commandArgv(command);
    return argv?.length === 2 && argv[0] === "bash" && argv[1] === `scripts/dev/${runner}.sh`;
  });
}

function workflowTriggerBound(text, integration) {
  const lines = text.split(/\r?\n/);
  const start = lines.findIndex((line) => /^on:\s*$/.test(line));
  if (start < 0) return false;
  let end = start + 1;
  while (end < lines.length && (!lines[end].trim() || /^\s/.test(lines[end]) || /^#/.test(lines[end]))) end++;
  const block = lines.slice(start + 1, end);
  const triggerIndex = block.findIndex((line) => /^  pull_request:\s*$/.test(line));
  if (triggerIndex < 0) return false;
  const tail = block.slice(triggerIndex + 1);
  const nextTrigger = tail.findIndex((line) => /^  [^ #]/.test(line));
  const pull = tail.slice(0, nextTrigger < 0 ? tail.length : nextTrigger);
  return !integration || pull.some((line) => /^    branches:\s*\[r3\/integration\]\s*$/.test(line));
}

// These are positive bindings for the already-required receipt consumers, not
// shell/env denylists. Comments cannot donate an executable always condition.
function alwaysReceiptConsumer(step) {
  const conditions = [...step.matchAll(/^(?:      - if:|        if:)\s*(.*)$/gm)]
    .map(([, value]) => value.split(/\s+#/)[0].trim());
  return conditions.length === 1 && ["always()", "${{ always() }}"].includes(conditions[0]);
}
function foundationReceiptConsumer(text) {
  const jobs = text.split(/^jobs:\s*$/m);
  const foundation = jobs.length === 2
    ? /^  foundation:\s*\n([\s\S]*?)(?=^  [^ #\n]|^\S|$(?![\s\S]))/m.exec(jobs[1])?.[1] : null;
  const steps = (foundation || "").split(/^    steps:\s*$/m)[1] || "";
  const consumers = steps.split(/(?=^      - )/m)
    .filter((step) => /name: Verify current-run matrix execution receipts/.test(step));
  return consumers.length === 1 && alwaysReceiptConsumer(consumers[0]) &&
    consumers[0].includes("env -u NODE_OPTIONS node scripts/dev/matrix-receipt-verify.mjs foundation");
}

export function matrixExecutionIssues({ node, workflow, registry, receipt = readFileSync("scripts/dev/matrix-receipt.mjs", "utf8"), foundationWorkflow = readFileSync(".github/workflows/foundation.yml", "utf8") }) {
  const issues = [];
  const active = (text) => text.replace(/\\\r?\n/g, " ").split(/\r?\n/)
    .map((line) => line.replace(/\s+#.*$/, "").trim()).filter((line) => line && !line.startsWith("#"));
  const nodeCommands = active(node).map(executableArgv).filter((argv) => argv?.[0] === "node" && argv.includes("--test"));
  const required = ["product-editor-matrix-ledger.test.mjs", "product-editor-matrix-execution.test.mjs", "product-editor-matrix-controls.test.mjs"];
  const receiptCalls = active(node).map(executableArgv).filter((argv) => argv?.[0] === "node" && argv[1] === "scripts/dev/matrix-receipt.mjs" && argv[2] === "node");
  if (receiptCalls.length !== 1) issues.push("test-node.sh missing one active matrix receipt runner");
  if (receiptCalls.some((argv) => argv.length !== 3)) issues.push("test-node.sh must not add matrix suite filters to its receipt invocation");
  for (const name of required) if (!receipt.includes(`tests/ci/${name}`)) issues.push(`matrix receipt suite set missing ${name}`);
  if (workflowHasSelectingGoFlags(foundationWorkflow))
    issues.push("foundation.yml GOFLAGS must not use Go test selection filters");
  if (workflowHasSelectingGoFlags(workflow))
    issues.push("gates.yml GOFLAGS must not use Go test selection filters");
  if (!workflowTriggerBound(foundationWorkflow, false)) issues.push("foundation.yml must retain its pull_request trigger");
  if (!workflowTriggerBound(workflow, true)) issues.push("gates.yml must retain pull_request targeting r3/integration");
  if (!workflow.includes("LC_MATRIX_RECEIPT_MODE: shard:unit") || !workflow.includes("node scripts/dev/matrix-receipt.mjs node"))
    issues.push("gates.yml must execute the matrix Node receipt in shard:unit");
  if (!workflow.includes('uses: actions/github-script@ed597411d8f924073f98dfc5c65a23a2325f34cd') ||
      !workflow.includes('"scripts/dev/matrix-receipt-verify.mjs", "required"') ||
      !workflow.includes('env: { ...process.env, NODE_OPTIONS: "" }'))
    issues.push("gates.yml required job must invoke the pinned JavaScript receipt verifier with NODE_OPTIONS cleared");
  if (!foundationReceiptConsumer(foundationWorkflow))
    issues.push("foundation.yml must always verify positive receipts with NODE_OPTIONS cleared");
  // Bind selectors to their real planner calls; a plan can cover every test yet
  // an extra exclusion at the live dispatch still prevents its execution.
  const dispatch = active(workflow).filter((line) => line.startsWith("shard:g[0-9][0-9]) "));
  const dynamicCatchAll = active(workflow).some((line) => line.startsWith("catch_all=") && line.includes("groups.find((g)=>g.catch_all).id"));
  const branch = dispatch[0] || "";
  const expectedSkip = 'LC_FOUNDATION_SKIP="$(node scripts/dev/shard-plan.mjs skip "' + "$" + '{MODE#shard:}")"';
  if (dispatch.length !== 1 || !dynamicCatchAll || !branch.includes('if [[ "${MODE#shard:}" == "$catch_all" ]]') ||
      !branch.includes('LC_MATRIX_RECEIPT_KIND=go LC_MATRIX_RECEIPT_MODE="$MODE"') ||
      !branch.includes('LC_FOUNDATION_RUN="$(node scripts/dev/shard-plan.mjs run "${MODE#shard:}")"') ||
      !branch.includes('LC_FOUNDATION_SKIP="$(node scripts/dev/shard-plan.mjs skip "${MODE#shard:}")"') ||
      branch.split(expectedSkip).length - 1 !== 2 ||
      !branch.includes("else LC_FOUNDATION_PKGS=./tests/foundation") || !branch.includes("bash scripts/dev/test-local.sh 2>&1"))
    issues.push("foundation CI must emit Go matrix receipts only for the source-derived catch-all shard and forward planner selectors");
  const jobs = workflow.split(/^jobs:\s*$/m);
  const gate = jobs.length === 2 ? /^  gate:\s*\n([\s\S]*?)(?=^  [^ #\n]|^\S|$(?![\s\S]))/m.exec(jobs[1])?.[1] : null;
  const continuation = [...(gate || "").matchAll(/^[ \t]+continue-on-error:[ \t]*(.*?)[ \t]*(?:#.*)?$/gim)];
  if (!gate || continuation.some(([, value]) => value !== "false"))
    issues.push("gates.yml gate job and test step must not continue after errors");
  const requiredJob = jobs.length === 2 ? /^  required:\s*\n([\s\S]*?)(?=^  [^ #\n]|^\S|$(?![\s\S]))/m.exec(jobs[1])?.[1] : null;
  const requiredHeader = (requiredJob || "").split(/^    steps:\s*$/m)[0];
  const requiredSteps = (requiredJob || "").split(/^    steps:\s*$/m)[1] || "";
  const requiredStepBlocks = requiredSteps.split(/(?=^      - )/m).filter((block) => block.trim());
  const resultConsumers = requiredStepBlocks.filter((block) => /name: Required jobs completed/.test(block));
  const receiptConsumers = requiredStepBlocks.filter((block) => /name: Verify current-run matrix execution receipts/.test(block) ||
    /actions\/github-script@ed597411d8f924073f98dfc5c65a23a2325f34cd/.test(block));
  const requiredJobContinuation = [...requiredHeader.matchAll(/^[ \t]+continue-on-error:[ \t]*(.*?)[ \t]*(?:#.*)?$/gim)];
  const requiredStepContinuations = requiredStepBlocks.flatMap((block) =>
    [...block.matchAll(/^[ \t]+continue-on-error:[ \t]*(.*?)[ \t]*(?:#.*)?$/gim),
      ...block.matchAll(/^      - continue-on-error:[ \t]*(.*?)[ \t]*(?:#.*)?$/gim)]
      .map(([, value]) => ({ block, value })));
  // The required job is the sole consumer of PLAN/GATE/AGG/DEPLOY and matrix
  // receipts. Every step that can affect its result is mandatory; only the
  // artifact download may be optional because the verifiers independently
  // fail when evidence is absent.
  const optionalArtifactDownload = (block) => /actions\/download-artifact@/.test(block);
  const invalidRequiredStepContinuation = requiredStepContinuations.some(({ block, value }) =>
    !(optionalArtifactDownload(block) && value === "true"));
  if (!requiredJob || requiredJobContinuation.some(([, value]) => value !== "false") || resultConsumers.length !== 1 || receiptConsumers.length !== 1 ||
      invalidRequiredStepContinuation ||
      !requiredHeader.includes("name: ${{ github.event_name == 'workflow_dispatch' && 'manual' || 'required' }}") ||
      !alwaysReceiptConsumer(receiptConsumers[0] || "") ||
      !receiptConsumers[0]?.includes('"scripts/dev/matrix-receipt-verify.mjs", "required"') ||
      !resultConsumers[0]?.includes('[[ "$GATE" == success ]]'))
    issues.push("gates.yml required job and both result/receipt consumers must remain mandatory and fail closed");
  if (!workflow.includes('GATE: ${{ needs.gate.result }}') || !workflow.includes('[[ "$GATE" == success ]]'))
    issues.push("gates.yml required aggregate must fail when any gate matrix leg fails");
  const foundation = modeEntries(registry).find((entry) => entry.name === "foundation");
  const commands = active(foundation?.run ?? "").filter((line) => /(?:^|\s)go test\s/.test(line));
  const selectors = ['${LC_FOUNDATION_RUN:+-run "$LC_FOUNDATION_RUN"}', '${LC_FOUNDATION_SKIP:+-skip "$LC_FOUNDATION_SKIP"}', '${LC_FOUNDATION_PKGS:-./...}'];
  const cleanCommands = commands.map((line) => ({ line, argv: executableArgv(selectors.reduce((clean, token) => clean.replace(token, ""), line)) }));
  if (commands.length !== 2 || cleanCommands.some(({ line, argv }) => selectors.some((token) => !line.includes(token)) ||
      !invokesActualGoTest(argv) ||
      hasGoSelectionFlags(argv) || hasSelectingGoFlags(commandArgv(line) ?? [])))
    issues.push("foundation go test must forward RUN, SKIP and package selectors without extra filters");
  const goCondition = "${{ !(github.event_name == 'pull_request' && github.base_ref == 'r3/integration') }}";
  for (const [runner, condition] of [["test-node", null], ["test-local", goCondition]]) {
    if (!foundationRunnerBound(foundationWorkflow, runner, condition))
      issues.push(`foundation.yml must execute scripts/dev/${runner}.sh in an enabled foundation workflow step`);
  }
  return issues;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const issues = matrixExecutionIssues({ node: readFileSync("scripts/dev/test-node.sh", "utf8"),
    foundationWorkflow: readFileSync(".github/workflows/foundation.yml", "utf8"),
    workflow: readFileSync(".github/workflows/gates.yml", "utf8"), registry: readFileSync("scripts/dev/test-local.sh", "utf8") });
  for (const issue of issues) console.error(`matrix-execution: ${issue}`);
  if (issues.length) process.exitCode = 1;
  else console.log("matrix-execution: active Node gates and foundation CI selector forwarding bound");
}
