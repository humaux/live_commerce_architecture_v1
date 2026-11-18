// Purpose: exercise the W1 HTTP CLI with synthetic loopback replies and intentional failures.
// Depends on: Node built-ins, the persisted corpus and check.mjs.
// Used by: local conformance harness checks before independent review.
import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { createHmac } from 'node:crypto';
import { once } from 'node:events';
import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';

const corpus = JSON.parse(readFileSync(new URL('../w1_corpus.json', import.meta.url), 'utf8'));
const cli = fileURLToPath(new URL('./check.mjs', import.meta.url));
const primary = corpus.cases.map((item) => ({
  raw: item.raw_body, signature: item.signature_header, tenant: item.untrusted_tenant_header,
  media: item.content_type, status: item.classification === 'invalid_signature' ? 403 : 200,
}));
const duplicate = corpus.cases.findIndex((item) => item.classification === 'duplicate_replay');

// Independently build the wire shape from the SQL oracle's supplemental fixture.
const first = primary.find((item) => item.status === 200);
const mid = JSON.parse(first.raw).entry[0].messaging[0].message.mid;
function supplemental(asset, text = 'synthetic-w1-fixed-body', status = 200) {
  const raw = JSON.stringify({ object: corpus.object, entry: [{ id: asset, time: 123, messaging: [
    { sender: { id: '4' }, recipient: { id: asset }, message: { mid, text } },
  ] }] });
  const signature = 'sha256=' + createHmac('sha256', corpus.hmac_secret).update(raw).digest('hex');
  return { ...first, raw, status, signature };
}

async function stub(t, fixtures, faultIndex = -1, fault) {
  const received = [];
  const server = createServer(async (req, res) => {
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    const index = received.length;
    received.push({
      method: req.method, path: req.url, raw: Buffer.concat(chunks),
      signature: req.headers['x-hub-signature-256'],
      tenant: req.headers['x-tenant-id'],
      media: req.headers['content-type'],
    });
    if (index === faultIndex) return fault(res);
    res.statusCode = fixtures[index]?.status ?? 500;
    res.end(res.statusCode === 200 ? 'EVENT_RECEIVED' : '');
  });
  t.after(() => {
    server.closeAllConnections();
    return new Promise((resolve) => server.close(resolve));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  return {
    received,
    run: (...flags) => promisify(execFile)(process.execPath, [cli, String(server.address().port), ...flags],
      { timeout: 15_000 }),
  };
}

function checkWire(received, fixtures) {
  assert.deepEqual(received, fixtures.map(({ raw, signature, tenant, media }) => ({
    method: 'POST', path: '/__l01/w1/webhook', raw: Buffer.from(raw), signature, tenant, media,
  })));
}

test('synthetic corpus and extended requests preserve wire bytes and accept expected replies', async (t) => {
  const missing = supplemental('1000000001901104');
  const inactive = supplemental('1000000001901103');
  const extended = [
    ...primary, supplemental('1000000001901102'), first,
    supplemental(corpus.authoritative_asset_id, 'synthetic-w1-changed-body', 409),
    missing, missing, inactive, inactive,
  ];
  for (const [fixtures, flags] of [[primary, []], [extended, ['--extended']]]) {
    const { received, run } = await stub(t, fixtures);
    const { stdout } = await run(...flags);
    assert.equal(stdout.trim(),
      `HTTP_RESPONSES_CHECKED requests=${fixtures.length} pg_readback=NOT_RUN commit_order=NOT_RUN`);
    checkWire(received, fixtures);
  }
});

for (const [name, index, reply, diagnostic] of [
  ['wrong status', 0, (res) => res.writeHead(202).end(), /expected HTTP 200, got 202/],
  ['duplicate with missing ACK', duplicate, (res) => res.end(), /duplicate_replay.*missing exact EVENT_RECEIVED ACK/],
  ['silently dropped duplicate', duplicate, () => {}, /duplicate_replay.*aborted/i],
]) {
  test(`synthetic ${name} fails the CLI`, async (t) => {
    const { received, run } = await stub(t, primary, index, reply);
    await assert.rejects(run(), (error) => {
      assert.equal(error.code, 1);
      assert.match(error.stderr, diagnostic);
      assert.equal(error.stdout, '');
      return true;
    });
    checkWire(received, primary.slice(0, index + 1));
  });
}
