// Drives the bench SQL via psql and verifies committed rows against signed wire inputs.
// Uses Node built-ins only. Adapter correctness and timing require separate runs.
import assert from 'node:assert/strict';
import { createCipheriv, createDecipheriv, createHash, createHmac, randomBytes, randomUUID, timingSafeEqual } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';

const read = (path) => readFileSync(new URL(path, import.meta.url), 'utf8');
const corpus = JSON.parse(read('../w1_corpus.json'));
assert.equal(corpus.synthetic_only, true);
const { app_id: app, object, authoritative_asset_id: primaryAsset, authoritative_tenant_id: tenant } = corpus;
const otherAsset = '1000000001901102';
const inactiveAsset = '1000000001901103';
const unknownAsset = '1000000001901104';
const bindings = [
  [primaryAsset, tenant, '10000000-0000-4000-8000-000000000001', '20000000-0000-4000-8000-000000000001', true],
  [otherAsset, '00000000-0000-4000-8000-0000000000c3', '10000000-0000-4000-8000-000000000002', '20000000-0000-4000-8000-000000000002', true],
  [inactiveAsset, tenant, '10000000-0000-4000-8000-000000000003', '20000000-0000-4000-8000-000000000003', false],
];
const keyId = 'neutral_w1_fixture';
const key = Buffer.alloc(32, 0x31);
const hash = (value) => createHash('sha256').update(value).digest('hex');
const mac = (raw) => createHmac('sha256', corpus.hmac_secret).update(raw).digest();

function verified({ raw_body: raw, signature_header: signature }) {
  if (signature.length !== 71 || !/^sha256=[0-9a-f]{64}$/.test(signature)) return null;
  if (!timingSafeEqual(mac(raw), Buffer.from(signature.slice(7), 'hex'))) return null;
  const body = JSON.parse(raw);
  const [entry] = body.entry;
  const [message] = entry.messaging;
  assert.equal(body.object, object);
  assert.equal(body.entry.length, 1);
  assert.equal(entry.messaging.length, 1);
  assert.equal(entry.id, message.recipient.id);
  return { asset: entry.id, mid: message.message.mid, digest: hash(raw), raw };
}

// Only signed wire bytes contribute expectations. Ignore supplied class, MID,
// expected deltas and aggregate counts. Fixtures below use the original miMessage shape.
const valid = corpus.cases.map(verified).filter(Boolean);
const unique = [...new Map(valid.map((e) => [e.mid, e])).values()];
assert.deepEqual([corpus.cases.length, valid.length, unique.length], [100, 80, 60]);
const digests = new Map(unique.map((e) => [e.mid, e.digest]));
assert.ok(valid.every((e) => e.digest === digests.get(e.mid)));
assert.ok(unique.every((e) => e.asset === primaryAsset));
const first = valid[0];

function supplemental(asset, mid) {
  const raw = JSON.stringify({ object, entry: [{ id: asset, time: 123, messaging: [
    { sender: { id: '4' }, recipient: { id: asset }, message: { mid, text: 'synthetic-w1-fixed-body' } },
  ] }] });
  return verified({ raw_body: raw, signature_header: 'sha256=' + mac(raw).toString('hex') });
}

const second = supplemental(otherAsset, first.mid);
const unscoped = [supplemental(unknownAsset, first.mid), supplemental(inactiveAsset, first.mid)];
const rollback = supplemental(primaryAsset, 'm.w1.rollback');
const prefix = (e) => [app, object, e.asset];
const quarantineTuple = (e) => [...prefix(e), e.mid, e.digest];
const scopedTuple = (e, binding) => [...prefix(e), ...binding.slice(1, 4), e.mid, e.digest];
const expectedPrimary = unique.map((e) => scopedTuple(e, bindings[0]));
const expectedExtended = [...expectedPrimary, scopedTuple(second, bindings[1])];
const canonical = (rows) => rows.map((row) => JSON.stringify(row)).sort();

function exactRows(actual, expected, label) {
  assert.deepEqual(canonical(actual), canonical(expected), label);
}

function checkPairs(inbox, intent, expected) {
  exactRows(inbox, expected, 'inbox rows differ');
  exactRows(intent, expected, 'linked intent rows differ');
}

function negativeExamples(inbox, intent, expected) {
  const wrong = inbox[0].with(3, corpus.spoofed_tenant_header);
  assert.throws(() => checkPairs(inbox.slice(1), intent.slice(1), expected), /inbox rows differ/);
  assert.throws(() => checkPairs([...inbox, inbox[0]], [...intent, intent[0]], expected), /inbox rows differ/);
  assert.throws(() => checkPairs([wrong, ...inbox.slice(1)], [wrong, ...intent.slice(1)], expected), /inbox rows differ/);
  assert.throws(() => checkPairs(inbox, intent.slice(1), expected), /linked intent rows differ/);
}

// Product v1 AES-256-GCM envelope and AAD layout. This bench stores exact
// wire bytes and uses its own event-key digest for this one-message corpus.
const aad = (id, e) => Buffer.from(JSON.stringify([
  'livecommerce/meta-payload/v1', 'quarantine', id, app, object, '',
  hash(JSON.stringify([...prefix(e), e.mid])), e.digest, '', '', '', 0, keyId,
]));

function seal(e) {
  const id = randomUUID();
  const nonce = randomBytes(12);
  const cipher = createCipheriv('aes-256-gcm', key, nonce);
  cipher.setAAD(aad(id, e));
  const ciphertext = Buffer.concat([cipher.update(e.raw), cipher.final(), cipher.getAuthTag()]);
  return [id, keyId, nonce, ciphertext];
}

function open(e, [id, actualKey, nonce, ciphertext]) {
  assert.equal(actualKey, keyId);
  const decipher = createDecipheriv('aes-256-gcm', key, nonce);
  decipher.setAAD(aad(id, e));
  decipher.setAuthTag(ciphertext.subarray(-16));
  return Buffer.concat([decipher.update(ciphertext.subarray(0, -16)), decipher.final()]);
}

const literal = (v) => Buffer.isBuffer(v)
  ? `decode('${v.toString('hex')}','hex')`
  : typeof v === 'boolean' ? String(v) : `'${v.replaceAll("'", "''")}'`;
const args = (values) => values.map(literal).join(',');
const admit = (e, envelope = []) => `SELECT l01_w1.admit(${args([...quarantineTuple(e), ...envelope])});\n`;

function psql(sql, expectedError) {
  const { PGHOST, PGHOSTADDR, PGSERVICE, PGOPTIONS, LC_TEST_DATABASE_ALLOWED } = process.env;
  assert.equal(LC_TEST_DATABASE_ALLOWED, '1', 'requires explicit disposable database opt-in');
  assert.ok(['127.0.0.1', '::1'].includes(PGHOST) && !PGHOSTADDR && !PGSERVICE && !PGOPTIONS,
    'requires explicit loopback PGHOST with no routing overrides');
  const run = spawnSync('psql', ['-XqAtw', '-h', PGHOST, '-d', 'lc_neutral_w1_test',
    '-v', 'ON_ERROR_STOP=1', '-v', 'VERBOSITY=verbose'], {
    input: `SET statement_timeout = '10s'; SET lock_timeout = '5s';
      SET standard_conforming_strings = on; SET default_transaction_isolation = 'read committed';\n${sql}`,
    encoding: 'utf8', timeout: 30_000,
  });
  if (run.error) throw run.error;
  if (expectedError) {
    assert.equal(run.status, 3, run.stderr);
    assert.ok(run.stderr.includes(expectedError), run.stderr);
    return [];
  }
  assert.equal(run.status, 0, run.stderr);
  return run.stdout.trim().split('\n').filter(Boolean);
}

function setup() {
  const version = Number(psql('SHOW server_version_num;')[0]);
  assert.ok(version >= 180000 && version < 190000, 'requires PostgreSQL 18');
  const rows = bindings.map((binding) => `(${args([app, object, ...binding])})`).join(',');
  psql(`BEGIN;\n${read('./schema.sql')}\nINSERT INTO l01_w1.binding VALUES ${rows};\nCOMMIT;`);
  console.log('W1 benchmark fixtures seeded on PostgreSQL ' + version);
}

function readback(extended) {
  const columns = 'i.app_id, i.object, i.asset_id, i.tenant_id, i.store_id, i.binding_id, i.external_key, i.body_sha256';
  const [inbox, intent, quarantine, registry] = psql(`
    BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
    SELECT coalesce(json_agg(json_build_array(${columns})), '[]') FROM l01_w1.inbox i;
    SELECT coalesce(json_agg(json_build_array(${columns})), '[]')
        FROM l01_w1.intent n LEFT JOIN l01_w1.inbox i ON i.id = n.inbox_id;
    SELECT coalesce(json_agg(json_build_array(app_id, object, asset_id, external_key, body_sha256,
        id, key_id, encode(nonce,'hex'), encode(ciphertext,'hex'))), '[]') FROM l01_w1.quarantine;
    SELECT coalesce(json_agg(json_build_array(app_id, object, asset_id, tenant_id, store_id, binding_id, active)),
        '[]') FROM l01_w1.binding;
    COMMIT;`).map(JSON.parse);

  const expected = extended ? expectedExtended : expectedPrimary;
  checkPairs(inbox, intent, expected);
  negativeExamples(inbox, intent, expected);
  exactRows(registry, bindings.map((binding) => [app, object, ...binding]), 'trusted bindings differ');
  const expectedQuarantine = extended ? unscoped : [];
  exactRows(quarantine.map((row) => row.slice(0, 5)), expectedQuarantine.map(quarantineTuple), 'quarantine rows differ');
  for (const [appId, obj, asset, mid, digest, id, actualKey, nonce, ciphertext] of quarantine) {
    const event = expectedQuarantine.find((e) => e.asset === asset && e.mid === mid && e.digest === digest);
    assert.equal(appId, app);
    assert.equal(obj, object);
    assert.deepEqual(open(event, [id, actualKey, Buffer.from(nonce, 'hex'), Buffer.from(ciphertext, 'hex')]), Buffer.from(event.raw));
  }
  console.log(`REAL_PG_READBACK pairs=${inbox.length} quarantine=${quarantine.length} rejected_mutations=4`);
}

function smoke() {
  setup();
  const seen = new Set();
  const statuses = valid.map((e) => {
    const result = seen.has(e.mid) ? 'replay' : 'routed';
    seen.add(e.mid);
    return result;
  });
  assert.deepEqual(psql(valid.map((e) => admit(e)).join('')), statuses);
  readback(false);
  const changed = { ...first, digest: hash(first.raw.replace('fixed-body', 'changed-body')) };
  assert.deepEqual(psql(admit(second) + admit(first) + admit(changed)), ['routed', 'replay', 'conflict']);
  for (const e of unscoped) {
    const sql = admit(e, seal(e));
    assert.deepEqual(psql('BEGIN;\n' + admit(e) + sql + 'COMMIT;\n' + sql),
      ['needs_quarantine', 'quarantined', 'quarantined']);
  }
  psql(`
    BEGIN;
    CREATE FUNCTION l01_w1.fail_intent() RETURNS trigger LANGUAGE plpgsql AS
      $$ BEGIN RAISE EXCEPTION 'W1 injected intent failure' USING ERRCODE = 'P1901'; END; $$;
    CREATE TRIGGER fail_intent BEFORE INSERT ON l01_w1.intent
      FOR EACH STATEMENT EXECUTE FUNCTION l01_w1.fail_intent();
    ${admit(rollback)}
    COMMIT;`, 'P1901: W1 injected intent failure');
  readback(true);
}

switch (process.argv[2]) {
  case 'setup': setup(); break;
  case 'readback': readback(false); break;
  case 'readback-extended': readback(true); break;
  case 'smoke': smoke(); break;
  case 'oracle': {
    checkPairs(expectedExtended, expectedExtended, expectedExtended);
    negativeExamples(expectedExtended, expectedExtended, expectedExtended);
    for (const event of unscoped) {
      const sealed = seal(event);
      assert.deepEqual(open(event, sealed), Buffer.from(event.raw));
      assert.throws(() => open({ ...event, mid: 'wrong-mid' }, sealed));
    }
    console.log('OFFLINE_ORACLE signed=80 bad_signature=20 unique=60 rejected_mutations=4 quarantine_authentication=checked');
    break;
  }
  default: throw new Error('expected setup, readback, readback-extended, smoke or oracle');
}
