// PS3: actual pinned Caddy validates/adapts the shipping config, then serves it
// against a container-local MOCK upstream. No production network or shared container.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFile, writeFile, mkdir, mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { request } from "node:https";
import path from "node:path";

const out = "output/platform-site",
  name = `lc-platform-ps3-${process.pid}`;
await mkdir(out, { recursive: true });
const dir = await mkdtemp(path.join(tmpdir(), "lc-platform-edge-"));
const image = (await readFile("deploy/docker/caddy.Dockerfile", "utf8")).match(
  /^ARG CADDY_IMAGE=(.+)$/m,
)[1];
const config = await readFile("deploy/caddy/Caddyfile", "utf8");
const vars = {
  LC_PLATFORM_HOST: "platform.localhost",
  LC_ADMIN_HOST: "admin.localhost",
  LC_STORE_HOST: "shop.localhost",
  LC_API_HOST: "api.localhost",
  LC_HOOKS_HOST: "hooks.localhost",
  ACME_EMAIL: "contact@example.invalid",
};
const args = Object.entries(vars).flatMap(([k, v]) => ["-e", `${k}=${v}`]);
const docker = (...a) =>
  execFileSync("docker", a, {
    encoding: "utf8",
    timeout: 90000,
    stdio: ["ignore", "pipe", "pipe"],
  });
let started = false;
const evidence = [];
try {
  const original = path.join(dir, "Caddyfile");
  await writeFile(original, config);
  const mounted = [
    "--rm",
    "--network",
    "none",
    "--tmpfs",
    "/run/caddy",
    ...args,
    "-v",
    `${original}:/etc/caddy/Caddyfile:ro`,
  ];
  await writeFile(
    `${out}/caddy-validate.txt`,
    docker(
      "run",
      ...mounted,
      image,
      "caddy",
      "validate",
      "--config",
      "/etc/caddy/Caddyfile",
      "--adapter",
      "caddyfile",
    ),
  );
  const adapted = docker(
    "run",
    ...mounted,
    image,
    "caddy",
    "adapt",
    "--config",
    "/etc/caddy/Caddyfile",
    "--adapter",
    "caddyfile",
    "--pretty",
  );
  const json = JSON.parse(adapted);
  assert.ok(json.apps.http);
  assert.ok(adapted.includes("platform.localhost"));
  assert.ok(adapted.includes('"status_code": 301'));
  await writeFile(`${out}/caddy-adapt.json`, adapted);
  assert.equal(
    docker("run", ...mounted, image, "caddy", "fmt", "/etc/caddy/Caddyfile"),
    config,
    "shipping Caddyfile formatting",
  );
  // Only append the local upstream, leaving all shipping host blocks unchanged.
  // Docker Desktop may retain a bind mount's old bytes after its host path is
  // rewritten. Mount a distinct immutable file for the runtime fixture.
  const runtime = path.join(dir, "Caddyfile.runtime");
  await writeFile(
    runtime,
    config +
      '\nhttp://:3100 {\n\tbind 127.0.0.1\n\trespond "PS3 MOCK upstream host={http.request.host}" 200\n}\n',
  );
  docker(
    "run",
    "-d",
    "--rm",
    "--name",
    name,
    "--tmpfs",
    "/run/caddy",
    "--tmpfs",
    "/data",
    "--tmpfs",
    "/config",
    ...args,
    "-v",
    `${runtime}:/etc/caddy/Caddyfile:ro`,
    "-p",
    "127.0.0.1::443",
    image,
    "caddy",
    "run",
    "--config",
    "/etc/caddy/Caddyfile",
    "--adapter",
    "caddyfile",
  );
  started = true;
  const port = Number(docker("port", name, "443/tcp").trim().split(":").at(-1));
  const edge = (host, url, method = "GET", headers = {}) =>
    new Promise((resolve, reject) => {
      const r = request(
        {
          hostname: "127.0.0.1",
          port,
          servername: host,
          path: url,
          method,
          headers: { host, ...headers },
          rejectUnauthorized: false,
          timeout: 5000,
        },
        (res) => {
          const chunks = [];
          res.on("data", (c) => chunks.push(c));
          res.on("end", () =>
            resolve({
              status: res.statusCode,
              headers: res.headers,
              text: Buffer.concat(chunks).toString(),
            }),
          );
        },
      );
      r.on("timeout", () => r.destroy(new Error("edge timeout")));
      r.on("error", reject);
      r.end();
    });
  for (let n = 0; n < 40; n++) {
    try {
      if ((await edge(vars.LC_PLATFORM_HOST, "/")).status === 200) break;
    } catch {}
    if (n === 39) throw new Error("Caddy not ready");
    await new Promise((r) => setTimeout(r, 250));
  }
  for (const url of [
    "/",
    "/privacy",
    "/terms",
    "/data-deletion",
    "/contact",
    "/en",
    "/zh-CN/contact",
    "/robots.txt",
    "/sitemap.xml",
    "/_next/static/test.css",
  ]) {
    const r = await edge(vars.LC_PLATFORM_HOST, url);
    assert.equal(r.status, 200, url);
    assert.equal(r.text, "PS3 MOCK upstream host=platform.localhost");
    evidence.push({ url, method: "GET", result: 200 });
  }
  assert.equal((await edge(vars.LC_PLATFORM_HOST, "/", "HEAD")).status, 200);
  // Review P2: only known public documents canonicalize; queries stay byte-for-byte.
  for (const url of [
    "/privacy/",
    "/terms/",
    "/data-deletion/",
    "/contact/",
    "/en/",
    "/zh-CN/",
    "/zh-TW/",
    "/en/privacy/",
    "/zh-CN/terms/",
    "/zh-TW/data-deletion/",
  ]) {
    for (const method of ["GET", "HEAD"]) {
      const query = "?from=review&x=%2F";
      const response = await edge(vars.LC_PLATFORM_HOST, url + query, method);
      assert.equal(response.status, 301, `${method} ${url}`);
      assert.equal(response.headers.location, url.slice(0, -1) + query);
      evidence.push({
        url: url + query,
        method,
        result: response.status,
        location: response.headers.location,
      });
    }
  }
  for (const url of ["/api/", "/site/", "/en/orders/", "/privacy//"]) {
    assert.equal((await edge(vars.LC_PLATFORM_HOST, url)).status, 404, url);
    evidence.push({ url, method: "GET", result: 404 });
  }
  assert.equal(
    (await edge(vars.LC_PLATFORM_HOST, "/privacy/", "POST")).status,
    404,
  );
  evidence.push({ url: "/privacy/", method: "POST", result: 404 });
  for (const url of [
    "/api",
    "/api/auth/login",
    "/api/auth/password/signup",
    "/api/stores",
    "/site/en/home",
    "/en/orders",
    "/en/../api/auth/login",
  ]) {
    assert.equal((await edge(vars.LC_PLATFORM_HOST, url)).status, 404, url);
    evidence.push({ url, result: 404 });
  }
  assert.equal(
    (await edge(vars.LC_PLATFORM_HOST, "/contact", "POST")).status,
    404,
  );
  assert.equal(
    (
      await edge(vars.LC_PLATFORM_HOST, "/api/auth/login", "GET", {
        "X-Forwarded-Host": vars.LC_ADMIN_HOST,
      })
    ).status,
    404,
  );
  const redirect = await edge(
    `www.${vars.LC_PLATFORM_HOST}`,
    "/en/privacy?from=review&x=1",
  );
  assert.equal(redirect.status, 301);
  assert.equal(
    redirect.headers.location,
    "https://platform.localhost/en/privacy?from=review&x=1",
  );
  assert.equal(
    (await edge(vars.LC_ADMIN_HOST, "/api/auth/login")).status,
    200,
    "admin BFF routing preserved",
  );
  await writeFile(
    `${out}/ps3-edge.json`,
    JSON.stringify(
      {
        tier: "LOCAL Caddy TLS / MOCK upstream",
        cases: evidence,
        head: 200,
        post: 404,
        spoofedForwardedHost: 404,
        www: redirect,
        admin: 200,
      },
      null,
      2,
    ),
  );
  console.log(
    `PASS PS3: pinned Caddy validate + fmt + adapt; ${evidence.length + 5} edge requests, www 301 path/query, actual Host preserved; MOCK upstream`,
  );
} finally {
  if (started) {
    await writeFile(`${out}/caddy-runtime.log`, docker("logs", name));
    docker("stop", "--time", "3", name);
  }
  await rm(dir, { recursive: true, force: true });
}
