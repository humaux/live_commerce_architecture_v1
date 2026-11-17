// Purpose: verify sequential HTTP replies for the signed W1 fixture corpus.
// Depends on: Node built-ins, the persisted corpus and the neutral PostgreSQL fixture contract.
// Used by: local W1 adapter conformance runs and check.test.mjs.
import assert from 'node:assert/strict';
import { createHmac } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { Agent, request } from 'node:http';

const { synthetic_only, hmac_secret, cases } =
  JSON.parse(readFileSync(new URL('../w1_corpus.json', import.meta.url), 'utf8'));
assert.equal(synthetic_only, true);
const endpoint = '/__l01/w1/webhook';

function extendedCases() {
  const first = cases.find((item) => item.expected_admit);
  const signed = (label, raw, status = 200) => ({
    classification: label, raw_body: raw, status,
    content_type: first.content_type, untrusted_tenant_header: first.untrusted_tenant_header,
    signature_header: 'sha256=' + createHmac('sha256', hmac_secret).update(raw).digest('hex'),
  });
  // Match pg/check.mjs supplemental() bytes, including its member order.
  const forAsset = (label, asset) => {
    const body = JSON.parse(first.raw_body);
    const [entry] = body.entry;
    entry.id = entry.messaging[0].recipient.id = asset;
    return signed(label, JSON.stringify(body));
  };
  const missing = forAsset('missing_binding', '1000000001901104');
  const inactive = forAsset('inactive_binding', '1000000001901103');
  return [
    forAsset('second_store_same_mid', '1000000001901102'),
    { ...first, classification: 'exact_redelivery' },
    signed('changed_digest', first.raw_body.replace('fixed-body', 'changed-body'), 409),
    missing, { ...missing, classification: 'missing_binding_redelivery' },
    inactive, { ...inactive, classification: 'inactive_binding_redelivery' },
  ];
}

function post(agent, port, fixture) {
  const body = Buffer.from(fixture.raw_body, 'utf8');
  return new Promise((resolve, reject) => {
    const req = request({
      hostname: '127.0.0.1', port, path: endpoint, method: 'POST', agent,
      signal: AbortSignal.timeout(3000),
      headers: {
        'Content-Type': fixture.content_type,
        'Content-Length': body.length,
        'X-Hub-Signature-256': fixture.signature_header,
        'X-Tenant-ID': fixture.untrusted_tenant_header,
      },
    }, async (res) => {
      try {
        let length = 0;
        const chunks = [];
        for await (const chunk of res) {
          length += chunk.length;
          assert.ok(length <= 512, 'response exceeded 512 bytes');
          chunks.push(chunk);
        }
        resolve({ status: res.statusCode, body: Buffer.concat(chunks) });
      } catch (error) {
        reject(error);
      }
    });
    req.on('error', reject);
    req.end(body);
  });
}

async function checkHTTP(port, extended = false) {
  assert.match(String(port), /^[1-9][0-9]{0,4}$/, 'explicit loopback port required');
  assert.ok(Number(port) <= 65535, 'loopback port out of range');
  const fixtures = extended ? [...cases, ...extendedCases()] : cases;
  const agent = new Agent();
  try {
    for (const [index, fixture] of fixtures.entries()) {
      const label = `request ${index + 1}/${fixtures.length} (${fixture.classification})`;
      try {
        const expectedStatus = fixture.status ?? (fixture.expected_admit ? 200 : 403);
        const reply = await post(agent, port, fixture);
        assert.equal(reply.status, expectedStatus, `expected HTTP ${expectedStatus}, got ${reply.status}`);
        if (expectedStatus === 200) {
          assert.ok(reply.body.equals(Buffer.from('EVENT_RECEIVED')), 'missing exact EVENT_RECEIVED ACK');
        }
      } catch (error) {
        throw new Error(`${label}: ${error.message}`, { cause: error });
      }
    }
    return fixtures.length;
  } finally {
    agent.destroy();
  }
}

if (import.meta.main) {
  const [port, mode, ...extra] = process.argv.slice(2);
  assert.ok(port && extra.length === 0 && (mode === undefined || mode === '--extended'),
    'usage: node bench/l01/neutral/http/check.mjs <loopback-port> [--extended]');
  const count = await checkHTTP(port, mode === '--extended');
  console.log(`HTTP_RESPONSES_CHECKED requests=${count} pg_readback=NOT_RUN commit_order=NOT_RUN`);
}
