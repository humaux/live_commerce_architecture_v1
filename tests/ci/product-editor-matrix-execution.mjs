// Purpose: Bind matrix source/evidence gates to active Node and foundation CI commands, independently of receipt hashes.
// Depends on: Node fs/url and the existing declarative mode parser; reads only repository source.
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
    else if (/\s/.test(char)) {
      if (word) argv.push(word);
      word = "";
    } else word += char;
  }
  if (quote) return null;
  if (word) argv.push(word);
  return argv;
}

export function matrixExecutionIssues({ node, workflow, registry }) {
  const issues = [];
  const active = (text) => text.replace(/\\\r?\n/g, " ").split(/\r?\n/)
    .map((line) => line.replace(/\s+#.*$/, "").trim()).filter((line) => line && !line.startsWith("#"));
  const nodeCommands = active(node).map(commandArgv).filter((argv) => argv?.[0] === "node" && argv.includes("--test"));
  const required = ["product-editor-matrix-ledger.test.mjs", "product-editor-matrix-execution.test.mjs", "product-editor-matrix-controls.test.mjs"];
  for (const name of required) {
    const token = `tests/ci/${name}`;
    if (!nodeCommands.some((argv) => argv.includes(token))) issues.push(`active node --test argv missing ${name}`);
  }
  if (nodeCommands.some((argv) => required.some((name) => argv.includes(`tests/ci/${name}`)) &&
      argv.some((arg) => ["--test-name-pattern", "--test-skip-pattern"].includes(arg.split("=")[0]))))
    issues.push("required matrix Node suites must not use test-name/test-skip filters");
  // Bind selectors to their real planner calls; a plan can cover every test yet
  // an extra exclusion at the live dispatch still prevents its execution.
  // CI-SPEED-522 has two scheduling lanes. Each owns exactly one active Gate
  // dispatch, bound to its own planner output; do not accept arbitrary duplicate
  // commands elsewhere in the workflow as proof that either lane executes it.
  const lanes = [["foundation_gate", "foundation_jobs"], ["gate", "gate_jobs"]];
  const sections = workflow.split(/^jobs:[ \t]*\r?$/m);
  const jobParts = sections.length === 2 ? sections[1].split(/^  ([A-Za-z_][\w-]*):[ \t]*\r?$/m) : [];
  const jobs = [];
  for (let i = 1; i < jobParts.length; i += 2) jobs.push({ name: jobParts[i], body: jobParts[i + 1] });
  const dispatches = (text) => active(text).filter((line) => line.startsWith("shard:g[0-9][0-9]) "));
  const command = 'shard:g[0-9][0-9]) LC_FOUNDATION_PKGS=./tests/foundation LC_FOUNDATION_RUN="$(node scripts/dev/shard-plan.mjs run "${MODE#shard:}")" LC_FOUNDATION_SKIP="$(node scripts/dev/shard-plan.mjs skip "${MODE#shard:}")" bash scripts/dev/test-local.sh 2>&1 | tee "$log" ;;';
  let bound = dispatches(workflow).length === lanes.length;
  for (const [name, output] of lanes) {
    const matchingJobs = jobs.filter((job) => job.name === name);
    const body = matchingJobs.length === 1 ? matchingJobs[0].body : "";
    const jobHeader = body.split(/^    steps:[ \t]*\r?$/m)[0];
    const jobEnv = /^    env:\r?\n((?: {6}[^\n]*(?:\n|$)|[ \t]*\r?\n)*)/m.exec(jobHeader)?.[1] ?? "";
    const gateSteps = body.split(/^      - /m).slice(1).filter((step) => /^        id: gate[ \t]*\r?$/m.test(step));
    const gateStep = gateSteps.length === 1 ? gateSteps[0] : "";
    // Keep this execution step's current configuration explicit. In particular,
    // step env would override job env, while its receipt reads the job's MODE.
    // New step-level configuration must be reviewed instead of silently trusted.
    const metadata = active(gateStep.split(/\r?\n/).filter((line, i) => i === 0 || /^ {8}\S/.test(line)).join("\n"));
    const run = /^        run: \|\r?\n((?: {10}[^\n]*(?:\n|$)|[ \t]*\r?\n)*)/m.exec(gateStep)?.[1] ?? "";
    const dispatch = dispatches(run);
    bound &&= active(jobHeader).filter((line) => line === `include: \${{ fromJSON(needs.plan.outputs.${output}) }}`).length === 1
      && active(jobEnv).filter((line) => line === "MODE: ${{ matrix.mode }}").length === 1
      && metadata.join("\n") === "name: Gate\nid: gate\nrun: |"
      && dispatch.length === 1 && dispatch[0] === command;
  }
  if (!bound) issues.push("foundation CI dispatch must forward only the real planner run/skip selectors in each planned lane's Gate step");
  const foundation = modeEntries(registry).find((entry) => entry.name === "foundation");
  const commands = active(foundation?.run ?? "").filter((line) => /(?:^|\s)go test\s/.test(line));
  const selectors = ['${LC_FOUNDATION_RUN:+-run "$LC_FOUNDATION_RUN"}', '${LC_FOUNDATION_SKIP:+-skip "$LC_FOUNDATION_SKIP"}', '${LC_FOUNDATION_PKGS:-./...}'];
  const extraArgv = commandArgv(selectors.reduce((line, token) => line.replace(token, ""), commands[0] ?? ""));
  if (commands.length !== 1 || selectors.some((token) => !commands[0].includes(token)) ||
      !extraArgv || extraArgv.some((arg) => ["-run", "-skip"].includes(arg.split("=")[0])))
    issues.push("foundation go test must forward RUN, SKIP and package selectors without extra filters");
  return issues;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const issues = matrixExecutionIssues({ node: readFileSync("scripts/dev/test-node.sh", "utf8"),
    workflow: readFileSync(".github/workflows/gates.yml", "utf8"), registry: readFileSync("scripts/dev/test-local.sh", "utf8") });
  for (const issue of issues) console.error(`matrix-execution: ${issue}`);
  if (issues.length) process.exitCode = 1;
  else console.log("matrix-execution: active Node gates and foundation CI selector forwarding bound");
}

