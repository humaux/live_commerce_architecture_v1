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

export function matrixExecutionIssues({ node, workflow, registry, foundationWorkflow = readFileSync(".github/workflows/foundation.yml", "utf8") }) {
  const issues = [];
  const active = (text) => text.replace(/\\\r?\n/g, " ").split(/\r?\n/)
    .map((line) => line.replace(/\s+#.*$/, "").trim()).filter((line) => line && !line.startsWith("#"));
  const nodeCommands = active(node).map(executableArgv).filter((argv) => argv?.[0] === "node" && argv.includes("--test"));
  const required = ["product-editor-matrix-ledger.test.mjs", "product-editor-matrix-execution.test.mjs", "product-editor-matrix-controls.test.mjs"];
  for (const name of required) {
    const token = `tests/ci/${name}`;
    if (!nodeCommands.some((argv) => argv.includes(token))) issues.push(`active node --test argv missing ${name}`);
  }
  if (workflowHasSelectingGoFlags(foundationWorkflow))
    issues.push("foundation.yml GOFLAGS must not use Go test selection filters");
  if (nodeCommands.some((argv) => required.some((name) => argv.includes(`tests/ci/${name}`)) &&
      argv.some((arg) => ["--test-name-pattern", "--test-skip-pattern", "--test-only", "--test-shard", "--test-concurrency"].includes(arg.split("=")[0]))))
    issues.push("required matrix Node suites must not use test selection/concurrency filters");
  // Bind selectors to their real planner calls; a plan can cover every test yet
  // an extra exclusion at the live dispatch still prevents its execution.
  const dispatch = active(workflow).filter((line) => line.startsWith("shard:g[0-9][0-9]) "));
  const prefix = 'shard:g[0-9][0-9]) LC_FOUNDATION_PKGS=./tests/foundation LC_FOUNDATION_RUN="$(node scripts/dev/shard-plan.mjs run "${MODE#shard:}")" LC_FOUNDATION_SKIP="$(node scripts/dev/shard-plan.mjs skip "${MODE#shard:}")" bash scripts/dev/test-local.sh 2>&1';
  if (dispatch.length !== 1 || !dispatch[0].startsWith(prefix)) issues.push("foundation CI dispatch must forward only the real planner run/skip selectors");
  const foundation = modeEntries(registry).find((entry) => entry.name === "foundation");
  const commands = active(foundation?.run ?? "").filter((line) => /(?:^|\s)go test\s/.test(line));
  const selectors = ['${LC_FOUNDATION_RUN:+-run "$LC_FOUNDATION_RUN"}', '${LC_FOUNDATION_SKIP:+-skip "$LC_FOUNDATION_SKIP"}', '${LC_FOUNDATION_PKGS:-./...}'];
  const extraArgv = executableArgv(selectors.reduce((line, token) => line.replace(token, ""), commands[0] ?? ""));
  if (commands.length !== 1 || selectors.some((token) => !commands[0].includes(token)) ||
      !extraArgv || extraArgv[0] !== "go" || extraArgv[1] !== "test" || hasGoSelectionFlags(extraArgv) || hasSelectingGoFlags(commandArgv(commands[0] ?? "") ?? []))
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
