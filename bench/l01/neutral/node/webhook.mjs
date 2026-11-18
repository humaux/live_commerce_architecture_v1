// Purpose: admit one signed synthetic W1 HTTP delivery through the frozen SQL function.
// Depends on: the portable corpus, Node crypto/HTTP, jsonc-parser and a pg pool.
// Used by: the fixture listener and direct caller controls.
// Invariants: PostgreSQL resolves scope and the HTTP ACK follows committed admission.
import { createCipheriv, createHash, createHmac, randomBytes, randomUUID, timingSafeEqual } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { visit } from 'jsonc-parser';

export const endpoint = '/__l01/w1/webhook';
const maxBody = 1 << 20;
const { app_id: app, object, hmac_secret: secret, synthetic_only: synthetic } =
  JSON.parse(readFileSync(new URL('../w1_corpus.json', import.meta.url), 'utf8'));
if (synthetic !== true) throw new Error('requires the synthetic W1 corpus');
const keyID = 'neutral_w1_fixture';
const key = Buffer.alloc(32, 0x31);
const utf8 = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true });
const sha = (value) => createHash('sha256').update(value).digest('hex');

function authenticated(raw, signatures) {
  if (signatures?.length !== 1) return false;
  const [signature] = signatures;
  if (signature.length !== 71 || !/^sha256=[0-9a-f]{64}$/.test(signature)) return false;
  return timingSafeEqual(
    createHmac('sha256', secret).update(raw).digest(),
    Buffer.from(signature.slice(7), 'hex'),
  );
}

function parseDelivery(raw) {
  const text = utf8.decode(raw);
  const payload = JSON.parse(text);
  // Match the reference's strict JSON names and Unicode escapes, including ignored members.
  const names = [];
  const reject = () => { throw new Error('invalid fixture JSON'); };
  visit(text, {
    onObjectBegin: () => names.push(new Set()),
    onObjectEnd: () => names.pop(),
    onObjectProperty(name) {
      const scope = names.at(-1);
      if (!name.isWellFormed() || scope.has(name)) reject();
      scope.add(name);
    },
    onLiteralValue(value) {
      if (typeof value === 'string' && !value.isWellFormed()) reject();
    },
    onError: reject,
  });

  if (payload?.object !== object || !Array.isArray(payload.entry) || payload.entry.length !== 1) return;
  const [entry] = payload.entry;
  if (!Array.isArray(entry?.messaging) || entry.messaging.length !== 1) return;
  const [message] = entry.messaging;
  const asset = entry.id;
  const mid = message?.message?.mid;
  if (typeof asset !== 'string' || asset.length < 1 || asset.length > 40 || /[^0-9]/.test(asset) ||
      asset !== message?.recipient?.id || typeof mid !== 'string' || !mid.length ||
      Buffer.byteLength(mid) > 1024 || mid.includes('\0')) return;
  return [asset, mid];
}

function seal(raw, asset, mid, digest) {
  const id = randomUUID();
  const nonce = randomBytes(12);
  const cipher = createCipheriv('aes-256-gcm', key, nonce);
  const eventKey = sha(JSON.stringify([app, object, asset, mid]));
  cipher.setAAD(Buffer.from(JSON.stringify([
    'livecommerce/meta-payload/v1', 'quarantine', id, app, object, '',
    eventKey, digest, '', '', '', 0, keyID,
  ])));
  return [id, keyID, nonce, Buffer.concat([cipher.update(raw), cipher.final(), cipher.getAuthTag()])];
}

async function admit(pool, raw, asset, mid) {
  const client = await pool.connect();
  let committed = false;
  try {
    await client.query('BEGIN ISOLATION LEVEL READ COMMITTED');
    const digest = sha(raw);
    const values = [app, object, asset, mid, digest];
    const first = await client.query('SELECT l01_w1.admit($1,$2,$3,$4,$5) AS status', values);
    let { status } = first.rows[0];

    if (status === 'needs_quarantine') {
      const envelope = seal(raw, asset, mid, digest);
      const sealed = await client.query('SELECT l01_w1.admit($1,$2,$3,$4,$5,$6,$7,$8,$9) AS status',
        [...values, ...envelope]);
      ({ status } = sealed.rows[0]);
      if (status !== 'quarantined') throw new Error('unexpected W1 quarantine result');
    } else if (!['routed', 'replay', 'conflict'].includes(status)) {
      throw new Error('unexpected W1 admission result');
    }

    const result = await client.query('COMMIT');
    if (result.command !== 'COMMIT') throw new Error('W1 transaction did not commit');
    committed = true;
    return status;
  } catch (error) {
    await client.query('ROLLBACK').catch(() => {});
    throw error;
  } finally {
    // A transport or timeout error can leave the session state unknown.
    client.release(!committed);
  }
}

export function webhook(pool) {
  return async (req, res) => {
    const reply = (status, body) => {
      res.statusCode = status;
      res.end(body);
    };
    if (req.url.split('?', 1)[0] !== endpoint) return reply(404);
    if (req.method !== 'POST') {
      res.setHeader('Allow', 'POST');
      return reply(405);
    }

    const chunks = [];
    let length = 0;
    try {
      for await (const chunk of req.iterator({ destroyOnReturn: false })) {
        length += chunk.length;
        if (length > maxBody) {
          res.setHeader('Connection', 'close');
          return reply(413);
        }
        chunks.push(chunk);
      }
    } catch {
      return reply(400);
    }

    const raw = Buffer.concat(chunks, length);
    if (!authenticated(raw, req.headersDistinct['x-hub-signature-256'])) return reply(403);
    let delivery;
    try {
      delivery = parseDelivery(raw);
    } catch {
      return reply(400);
    }
    if (!delivery) return reply(400);

    try {
      const status = await admit(pool, raw, ...delivery);
      if (status === 'conflict') return reply(409);
      return reply(200, 'EVENT_RECEIVED');
    } catch {
      return reply(503);
    }
  };
}
