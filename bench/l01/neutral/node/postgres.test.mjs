// Purpose: run frozen wire and restricted readback checks and verify the real PG commit boundary.
// Depends on: a fresh disposable PostgreSQL 18 cluster, the frozen JS oracles and the normal Node listener.
// Used by: the integrator, once on a fresh cluster for each corpus mode.
import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { createHash, createHmac, randomBytes } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';
import test from 'node:test';
import { Client } from 'pg';
import { fixtureConfig, startFixture } from './main.mjs';

const run = promisify(execFile);
const corpus = JSON.parse(readFileSync(new URL('../w1_corpus.json', import.meta.url), 'utf8'));
const sample = corpus.cases.find((item) => item.expected_admit);
const ingressRole = 'l01_w1_ingress';
const readerRole = 'l01_w1_reader';

async function check(path, args, credentials) {
  const keys = ['PATH', 'HOME', 'TMPDIR'];
  if (credentials) {
    keys.push('PGHOST', 'PGPORT', 'LC_TEST_DATABASE_ALLOWED', 'LC_W1_PG_CONTAINER',
      'DOCKER_HOST', 'DOCKER_CONTEXT', 'DOCKER_CONFIG');
    if (credentials.PGUSER === 'postgres') keys.push('PGPASSFILE');
  }
  const env = Object.fromEntries(keys.filter((key) => process.env[key] !== undefined)
    .map((key) => [key, process.env[key]]));
  const { stdout } = await run(process.execPath, [fileURLToPath(new URL(path, import.meta.url)), ...args], {
    env: { ...env, ...credentials }, timeout: 35_000,
  });
  console.log(stdout.trim());
}

async function connect(t, role, password) {
  const client = new Client(fixtureConfig(role, password));
  await client.connect();
  t.after(() => client.end());
  return client;
}

function send(url, body, controller) {
  let observed = false;
  const reply = fetch(url, {
    method: 'POST', body, redirect: 'error',
    signal: AbortSignal.any([AbortSignal.timeout(7000), ...(controller ? [controller.signal] : [])]),
    headers: {
      'Content-Type': sample.content_type,
      'X-Tenant-ID': sample.untrusted_tenant_header,
      'X-Hub-Signature-256': 'sha256=' + createHmac('sha256', corpus.hmac_secret).update(body).digest('hex'),
    },
  }).then(async (response) => {
    observed = true;
    return { status: response.status, body: await response.text() };
  });
  // Fault requests can finish before the control awaits them.
  reply.catch(() => {});
  return { reply, observed: () => observed };
}

async function expectReply(request, expected) {
  const { status, body } = await request.reply;
  assert.equal(status, expected);
  assert.equal(body === 'EVENT_RECEIVED', expected === 200);
}

function fixtureBody(mid, asset = corpus.authoritative_asset_id) {
  return sample.raw_body.replace(sample.event_id, mid).replaceAll(corpus.authoritative_asset_id, asset);
}

async function waitFor(read, expected, label) {
  const deadline = performance.now() + 1500;
  let actual;
  do {
    actual = await read();
    if (actual === expected) return;
    await delay(10);
  } while (performance.now() < deadline);
  assert.equal(actual, expected, label);
}

async function assertCommitControls(t, admin, reader, url) {
  // The trigger runs after admit returned its provisional result, during real COMMIT.
  await admin.query(`
    CREATE FUNCTION l01_w1.test_node_commit_boundary() RETURNS trigger LANGUAGE plpgsql AS $$
    BEGIN
      IF NEW.external_key = 'm.w1.node.commit-failure' THEN
        RAISE EXCEPTION 'fixture commit rejected' USING ERRCODE = 'P1902';
      ELSIF NEW.external_key IN ('m.w1.node.concurrent', 'm.w1.node.lost-ack') THEN
        PERFORM pg_advisory_xact_lock(1901118);
      END IF;
      RETURN NEW;
    END;
    $$;
    CREATE CONSTRAINT TRIGGER test_node_commit_boundary AFTER INSERT ON l01_w1.inbox
      DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION l01_w1.test_node_commit_boundary();
  `);

  const pair = async (mid) => (await reader.query(`
    SELECT count(*)::int AS inbox, count(n.inbox_id)::int AS intent, min(i.body_sha256) AS digest
    FROM l01_w1.inbox i LEFT JOIN l01_w1.intent n ON i.id = n.inbox_id
    WHERE i.external_key = $1`, [mid])).rows[0];
  const absent = { inbox: 0, intent: 0, digest: null };
  const committed = (body) => ({ inbox: 1, intent: 1, digest: createHash('sha256').update(body).digest('hex') });
  const waiters = (event, count) => waitFor(async () => (await admin.query(`
    SELECT count(*)::int AS count FROM pg_stat_activity
    WHERE datname = current_database() AND usename = 'l01_w1_ingress'
      AND application_name = 'l01-w1-node' AND wait_event_type = 'Lock' AND wait_event = $1`,
  [event])).rows[0].count, count, `waiting at PostgreSQL ${event} boundary`);
  const locked = async (control) => {
    assert.equal((await admin.query('SELECT pg_try_advisory_lock(1901118) AS ok')).rows[0].ok, true);
    try {
      return await control();
    } finally {
      assert.equal((await admin.query('SELECT pg_advisory_unlock(1901118) AS ok')).rows[0].ok, true);
    }
  };

  await t.test('failed COMMIT produces 503 and no surviving pair', async () => {
    const mid = 'm.w1.node.commit-failure';
    await expectReply(send(url, fixtureBody(mid)), 503);
    assert.deepEqual(await pair(mid), absent);
  });

  await t.test('held COMMIT delays ACK and concurrent exact/conflicting requests wait on the key', async () => {
    const mid = 'm.w1.node.concurrent';
    const body = fixtureBody(mid);
    const [first, replay, conflict] = await locked(async () => {
      const first = send(url, body);
      await waiters('advisory', 1);
      assert.equal(first.observed(), false);
      const replay = send(url, body);
      const conflict = send(url, body.replace('fixed-body', 'changed-body'));
      await waiters('transactionid', 2);
      assert.deepEqual(await pair(mid), absent);
      assert.deepEqual([first, replay, conflict].map((request) => request.observed()), [false, false, false]);
      return [first, replay, conflict];
    });
    await expectReply(first, 200);
    assert.deepEqual(await pair(mid), committed(body));
    await expectReply(replay, 200);
    await expectReply(conflict, 409);
    assert.deepEqual(await pair(mid), committed(body));
  });

  await t.test('client disconnect before ACK permits exact redelivery of the committed pair', async () => {
    const mid = 'm.w1.node.lost-ack';
    const body = fixtureBody(mid);
    await locked(async () => {
      const controller = new AbortController();
      const original = send(url, body, controller);
      await waiters('advisory', 1);
      assert.deepEqual(await pair(mid), absent);
      assert.equal(original.observed(), false);
      controller.abort();
      await assert.rejects(original.reply, { name: 'AbortError' });
    });
    await waitFor(async () => (await pair(mid)).digest, committed(body).digest, 'original commit after disconnect');
    await expectReply(send(url, body), 200);
    assert.deepEqual(await pair(mid), committed(body));
  });
}

test('Node fixture with frozen HTTP and restricted PostgreSQL readback', { timeout: 95_000 }, async (t) => {
  const mode = process.env.LC_W1_NODE_MODE;
  assert.ok(mode === 'base' || mode === 'extended', 'select base or extended on a fresh cluster');
  const extended = mode === 'extended';
  assert.equal(process.env.PGUSER, 'postgres', 'requires the disposable fixture administrator');
  const admin = await connect(t, 'postgres', process.env.PGPASSWORD);
  const { rows: [identity] } = await admin.query(`
    SELECT current_setting('server_version_num')::int AS version,
      session_user = 'postgres' AND current_user = session_user
        AND current_database() = 'lc_neutral_w1_test'
        AND to_regnamespace('l01_w1') IS NULL
        AND NOT EXISTS (
          SELECT 1 FROM pg_roles WHERE rolname IN ('l01_w1_ingress','l01_w1_reader','l01_w1_owner')
        ) AS fresh`);
  assert.ok(identity.fresh && identity.version >= 180000 && identity.version < 190000,
    'requires a fresh PostgreSQL 18 cluster and empty fixture database');

  await check('../pg/check.mjs', ['setup'], { PGUSER: 'postgres', PGPASSWORD: process.env.PGPASSWORD });
  const [ingressPassword, readerPassword] = [0, 1].map(() => randomBytes(24).toString('hex'));
  // Generated hex credentials are scoped to this disposable cluster.
  await admin.query(`ALTER ROLE l01_w1_ingress LOGIN PASSWORD '${ingressPassword}';
    ALTER ROLE l01_w1_reader LOGIN PASSWORD '${readerPassword}';`);

  const reader = await connect(t, readerRole, readerPassword);
  const { server, pool } = await startFixture(0, ingressPassword);
  t.after(async () => {
    await new Promise((resolve) => {
      server.close(resolve);
      server.closeAllConnections();
    });
    await pool.end();
  });
  const { address, port } = server.address();
  assert.equal(address, '127.0.0.1');
  const url = `http://${address}:${port}/__l01/w1/webhook`;
  await assert.rejects(pool.query('SELECT * FROM l01_w1.inbox LIMIT 0'), { code: '42501' });
  await assert.rejects(reader.query('INSERT INTO l01_w1.inbox DEFAULT VALUES'), { code: '42501' });
  await assert.rejects(reader.query('SELECT l01_w1.admit(NULL,NULL,NULL,NULL,NULL)'), { code: '42501' });

  await check('../http/check.mjs', [String(port), ...(extended ? ['--extended'] : [])]);
  await check('../pg/check.mjs', [extended ? 'readback-extended' : 'readback'], {
    PGUSER: readerRole, PGPASSWORD: readerPassword,
  });

  // Additional MIDs leave the frozen exact row comparison above untouched.
  if (extended) {
    await t.test('first quarantine envelope survives staged redelivery', async () => {
      const mid = 'm.w1.node.staged-quarantine';
      for (const asset of ['1000000001901104', '1000000001901103']) {
        const body = fixtureBody(mid, asset);
        const readEnvelope = async () => (await reader.query(`
          SELECT id, key_id, nonce, ciphertext FROM l01_w1.quarantine
          WHERE asset_id = $1 AND external_key = $2`, [asset, mid])).rows;
        await expectReply(send(url, body), 200);
        const original = await readEnvelope();
        assert.equal(original.length, 1);
        await expectReply(send(url, body), 200);
        assert.deepEqual(await readEnvelope(), original);
      }
      assert.equal((await reader.query('SELECT count(*)::int AS count FROM l01_w1.inbox WHERE external_key = $1',
        [mid])).rows[0].count, 0);
    });
  }
  await assertCommitControls(t, admin, reader, url);
});
