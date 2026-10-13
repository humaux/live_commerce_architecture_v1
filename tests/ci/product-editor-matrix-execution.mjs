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
  const dispatch = active(workflow).filter((line) => line.startsWith("shard:g[0-9][0-9]) "));
  const prefix = 'shard:g[0-9][0-9]) LC_FOUNDATION_PKGS=./tests/foundation LC_FOUNDATION_RUN="$(node scripts/dev/shard-plan.mjs run "${MODE#shard:}")" LC_FOUNDATION_SKIP="$(node scripts/dev/shard-plan.mjs skip "${MODE#shard:}")" bash scripts/dev/test-local.sh 2>&1';
  if (dispatch.length !== 1 || !dispatch[0].startsWith(prefix)) issues.push("foundation CI dispatch must forward only the real planner run/skip selectors");
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
