// Purpose: drive the W1 handler with signed wire bytes and a scripted PostgreSQL client.
// Depends on: the portable corpus and the same handler as the fixture listener.
// Used by: targeted Node checks for pre-write rejection, quarantine and commit ordering.
import assert from 'node:assert/strict';
import { createHash, createHmac } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { Readable } from 'node:stream';
import { setImmediate } from 'node:timers/promises';
import test from 'node:test';
import { endpoint, webhook } from './webhook.mjs';

const corpus = JSON.parse(readFileSync(new URL('../w1_corpus.json', import.meta.url), 'utf8'));
const sample = corpus.cases.find((entry) => entry.expected_admit);
const raw = Buffer.from(sample.raw_body);
const sign = (bytes) => 'sha256=' + createHmac('sha256', corpus.hmac_secret).update(bytes).digest('hex');

function invoke(pool, bytes = raw, signatures = [sign(bytes)]) {
  const req = Object.assign(Readable.from([bytes]), {
    url: endpoint,
    method: 'POST',
    headersDistinct: { 'x-hub-signature-256': signatures, 'x-tenant-id': [sample.untrusted_tenant_header] },
  });
  const replies = [];
  const res = {
    setHeader() {},
    end(body) { replies.push([this.statusCode, body]); },
  };
  return { replies, handled: webhook(pool)(req, res) };
}

test('bad signatures, invalid signed deliveries and oversized bodies make zero persistence calls', async () => {
  const bad = corpus.cases.find((entry) => !entry.expected_admit);
  const malformedBodies = [
    Buffer.from('{'),
    Buffer.concat([raw.subarray(0, -1), Buffer.from([0xff, 0x7d])]),
    Buffer.from(sample.raw_body.replace('"object":', '"object":"page","obj\\u0065ct":')),
    Buffer.from(sample.raw_body.replace('fixed-body', '\\ud800')),
    Buffer.from(sample.raw_body.replace('"recipient":{"id":"', '"recipient":{"id":"9')),
    Buffer.from(sample.raw_body.replace(sample.event_id, 'é'.repeat(513))),
  ];
  const cases = [
    [Buffer.from(bad.raw_body), [bad.signature_header], 403],
    [Buffer.from('{'), [], 403],
    [raw, [sign(raw), sign(raw)], 403],
    ...malformedBodies.map((body) => [body, [sign(body)], 400]),
    [Buffer.alloc((1 << 20) + 1), [], 413],
  ];
  let connections = 0;
  const pool = { connect() { connections++; throw new Error('unexpected database call'); } };
  for (const [bytes, signatures, expected] of cases) {
    const { handled, replies } = invoke(pool, bytes, signatures);
    await handled;
    assert.deepEqual(replies, [[expected, undefined]]);
  }
  assert.equal(connections, 0);
});

test('a provisional routed result produces no reply until COMMIT resolves (early ACK negative)', async () => {
  const waiting = Promise.withResolvers();
  const commit = Promise.withResolvers();
  const queries = [];
  const client = {
    async query(sql, values) {
      queries.push([sql, values]);
      if (sql === 'COMMIT') {
        waiting.resolve();
        return commit.promise;
      }
      return { rows: [{ status: 'routed' }] };
    },
    release(discard) { assert.equal(discard, false); },
  };

  const { handled, replies } = invoke({ connect: async () => client });
  try {
    await waiting.promise;
    await setImmediate();
    assert.deepEqual(replies, [], 'HTTP replied before COMMIT resolved');
    assert.deepEqual(queries, [
      ['BEGIN ISOLATION LEVEL READ COMMITTED', undefined],
      ['SELECT l01_w1.admit($1,$2,$3,$4,$5) AS status', [
        corpus.app_id, corpus.object, corpus.authoritative_asset_id, sample.event_id,
        createHash('sha256').update(raw).digest('hex'),
      ]],
      ['COMMIT', undefined],
    ]);
  } finally {
    commit.resolve({ command: 'COMMIT' });
    await handled;
  }
  assert.deepEqual(replies, [[200, 'EVENT_RECEIVED']]);
});

test('a failed COMMIT or ROLLBACK tag returns 503, rolls back and discards the client', async () => {
  for (const command of [undefined, 'ROLLBACK']) {
    const actions = [];
    const client = {
      async query(sql) {
        actions.push(sql.split(' ')[0]);
        if (sql === 'COMMIT' && !command) throw new Error('deferred constraint failed');
        return { command, rows: [{ status: 'routed' }] };
      },
      release(discard) { actions.push(discard); },
    };
    const { handled, replies } = invoke({ connect: async () => client });
    await handled;
    assert.deepEqual(replies, [[503, undefined]]);
    assert.deepEqual(actions, ['BEGIN', 'SELECT', 'COMMIT', 'ROLLBACK', true]);
  }
});

test('each quarantine attempt probes and seals in the same transaction with fresh randomness', async () => {
  const asset = '1000000001901104';
  const bytes = Buffer.from(sample.raw_body.replaceAll(corpus.authoritative_asset_id, asset));
  const envelopes = [];
  for (let i = 0; i < 2; i++) {
    const calls = [];
    const client = {
      async query(sql, values) {
        calls.push(sql.split(' ')[0]);
        if (values?.length === 5) return { rows: [{ status: 'needs_quarantine' }] };
        if (values?.length === 9) {
          assert.deepEqual(values.slice(0, 5), [corpus.app_id, corpus.object, asset, sample.event_id,
            createHash('sha256').update(bytes).digest('hex')]);
          envelopes.push(values.slice(5));
          return { rows: [{ status: 'quarantined' }] };
        }
        return { command: sql };
      },
      release(discard) { calls.push(discard); },
    };
    const { handled, replies } = invoke({ connect: async () => client }, bytes);
    await handled;
    assert.deepEqual(replies, [[200, 'EVENT_RECEIVED']]);
    assert.deepEqual(calls, ['BEGIN', 'SELECT', 'SELECT', 'COMMIT', false]);
  }
  assert.notEqual(envelopes[0][0], envelopes[1][0]);
  assert.notDeepEqual(envelopes[0][2], envelopes[1][2]);
  for (const [, keyID, nonce, ciphertext] of envelopes) {
    assert.equal(keyID, 'neutral_w1_fixture');
    assert.equal(nonce.length, 12);
    assert.equal(ciphertext.length, bytes.length + 16);
  }
});
