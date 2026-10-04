// PS3: actual pinned Caddy validates/adapts the shipping config, then serves it
// against a container-local MOCK upstream. No production network or shared container.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFile, writeFile, mkdir, mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { request } from "node:https";
import { isIP } from "node:net";
import path from "node:path";

const out = process.env.LC_PLATFORM_EDGE_EVIDENCE ?? "output/platform-site",
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
      '\nhttp://:3100 {\n\tbind 127.0.0.1\n\trespond "PS3 MOCK upstream host={http.request.host}" 200\n}\n' +
      // MOCK observes actual upstream headers only. Its allowlist is a fixture,
      // not proof that the production BFF authenticates an edge independently.
      `\nhttp://:3200 {\n\tbind 127.0.0.1\n\t@known host ${vars.LC_STORE_HOST}\n\thandle @known {\n\t\trespond \`{"host":"{http.request.host}","xff":"{http.request.header.X-Forwarded-For}","xfh":"{http.request.header.X-Forwarded-Host}"}\` 200\n\t}\n\thandle {\n\t\trespond \`{"host":"{http.request.host}","xff":"{http.request.header.X-Forwarded-For}","xfh":"{http.request.header.X-Forwarded-Host}"}\` 404\n\t}\n}\n`,
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
  const storefront = [];
  // Access logs measure the actual edge connection peer, independently of the
  // echoed forwarded header. Docker's peer is not assumed to be loopback.
  const connectionPeer = async (uri) => {
    for (let attempt = 0; attempt < 20; attempt++) {
      const records = docker("logs", name)
        .split("\n")
        .flatMap((line) => {
          try {
            return [JSON.parse(line)];
          } catch {
            return [];
          }
        });
      const record = records.find((entry) => entry.request?.uri === uri);
      if (record) {
        assert.ok(
          isIP(record.request.remote_ip),
          "access log has a literal connection peer",
        );
        return record.request.remote_ip;
      }
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    throw new Error(`missing Caddy connection-peer access log: ${uri}`);
  };
  for (const [label, hostileXFF] of [
    ["single", "203.0.113.9"],
    ["chain", "203.0.113.9, 198.51.100.8"],
  ]) {
    const uri = `/r10-edge/${label}`;
    const response = await edge(vars.LC_STORE_HOST, uri, "GET", {
      "X-Forwarded-For": hostileXFF,
      "X-Forwarded-Host": vars.LC_ADMIN_HOST,
    });
    assert.equal(response.status, 200, `storefront ${label} request`);
    const upstream = JSON.parse(response.text);
    const peer = await connectionPeer(uri);
    assert.equal(
      upstream.host,
      vars.LC_STORE_HOST,
      "actual store Host survives hostile XFH",
    );
    assert.equal(upstream.xfh, vars.LC_STORE_HOST, "edge replaces hostile XFH");
    assert.equal(
      upstream.xff,
      peer,
      "XFF is exactly the connection peer, never the buyer value or appended chain",
    );
    assert.notEqual(upstream.xff, hostileXFF);
    storefront.push({
      label,
      hostileXFF,
      hostileXFH: vars.LC_ADMIN_HOST,
      peer,
      status: response.status,
      upstream,
    });
  }
  // Keep known SNI so this request reaches the shipping catch-all with an
  // unknown actual HTTP Host. Its MOCK upstream must see that Host and reject
  // it even when the buyer supplies the real store in X-Forwarded-Host.
  const unknownHost = "unknown.localhost";
  const unknownURI = "/r10-edge/unknown-http-host";
  const unknown = await edge(vars.LC_STORE_HOST, unknownURI, "GET", {
    host: unknownHost,
    "X-Forwarded-Host": vars.LC_STORE_HOST,
    "X-Forwarded-For": "203.0.113.9, 198.51.100.8",
  });
  assert.equal(
    unknown.status,
    404,
    "unknown actual Host rejected by MOCK allowlist through shipping catch-all",
  );
  const unknownUpstream = JSON.parse(unknown.text);
  const unknownPeer = await connectionPeer(unknownURI);
  assert.equal(
    unknownUpstream.host,
    unknownHost,
    "XFH cannot turn unknown actual Host into the store",
  );
  assert.equal(unknownUpstream.xfh, unknownHost);
  assert.equal(
    unknownUpstream.xff,
    unknownPeer,
    "catch-all also replaces buyer XFF with connection peer",
  );
  storefront.push({
    label: "unknown-http-host-known-sni",
    hostileXFH: vars.LC_STORE_HOST,
    peer: unknownPeer,
    status: unknown.status,
    upstream: unknownUpstream,
    boundary: "MOCK allowlist rejection; no real BFF authority claim",
  });
  let tlsDenial;
  await assert.rejects(
    edge(unknownHost, "/r10-edge/unknown-sni", "GET", {
      "X-Forwarded-Host": vars.LC_STORE_HOST,
    }),
    (error) => {
      assert.equal(
        error.code,
        "EPROTO",
        "unknown SNI fails during TLS, not an unrelated timeout",
      );
      assert.match(
        error.message,
        /(?:tlsv1 alert internal error|sslv3 alert handshake failure)/i,
      );
      tlsDenial = { code: error.code, message: error.message };
      return true;
    },
  );
  storefront.push({
    label: "unknown-sni",
    host: unknownHost,
    hostileXFH: vars.LC_STORE_HOST,
    result: "TLS_DENIED",
    observed: tlsDenial,
    boundary:
      "shipping on-demand TLS with no local ask service; not a domain-authorization service test",
  });
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
        storefront,
      },
      null,
      2,
    ),
  );
  console.log(
    `PASS PS3: pinned Caddy validate + fmt + adapt; ${evidence.length + 5} platform edge requests + ${storefront.length} storefront edge cases, www 301 path/query, actual Host preserved and XFF equals connection peer; MOCK upstream`,
  );
} finally {
  if (started) {
    await writeFile(`${out}/caddy-runtime.log`, docker("logs", name));
    docker("stop", "--time", "3", name);
  }
  await rm(dir, { recursive: true, force: true });
}
