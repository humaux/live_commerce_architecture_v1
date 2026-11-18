// Purpose: start the loopback-only W1 fixture listener and restricted PostgreSQL pool.
// Depends on: Node HTTP, node-postgres, and the frozen l01_w1 fixture on PostgreSQL 18.
// Used by: the CLI and real-PostgreSQL correctness runner.
import { once } from 'node:events';
import { createServer } from 'node:http';
import { Pool } from 'pg';
import { webhook } from './webhook.mjs';

const host = '127.0.0.1';
const database = 'lc_neutral_w1_test';
const ingressRole = 'l01_w1_ingress';

function fixturePort(value) {
  if (!/^[1-9][0-9]{0,4}$/.test(value) || String(Number(value)) !== value || Number(value) > 65535) {
    throw new Error('requires an explicit TCP port from 1 to 65535');
  }
  return Number(value);
}

export function fixtureConfig(user, password) {
  const { LC_TEST_DATABASE_ALLOWED, PGHOST, PGHOSTADDR, PGSERVICE, PGOPTIONS, PGPORT } = process.env;
  if (LC_TEST_DATABASE_ALLOWED !== '1' || PGHOST !== host || PGHOSTADDR || PGSERVICE || PGOPTIONS) {
    throw new Error('W1 requires disposable database opt-in and literal loopback PGHOST with no routing overrides');
  }
  return {
    host, port: fixturePort(PGPORT), database, user, password, ssl: false, client_encoding: 'UTF8',
    connectionTimeoutMillis: 2000, statement_timeout: 4500, query_timeout: 5000,
  };
}

export async function startFixture(port, password = process.env.PGPASSWORD) {
  const pool = new Pool({
    ...fixtureConfig(ingressRole, password),
    application_name: 'l01-w1-node',
    max: 8,
    onConnect: async (client) => {
      const { rows: [row] } = await client.query(`
        SELECT session_user = 'l01_w1_ingress' AND current_user = session_user
          AND current_database() = 'lc_neutral_w1_test'
          AND current_setting('server_version_num')::int BETWEEN 180000 AND 189999
          AND NOT (rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole) AS valid
        FROM pg_roles WHERE rolname = session_user`);
      if (!row?.valid) throw new Error('W1 requires the restricted ingress login on PostgreSQL 18');
    },
  });
  pool.on('error', () => console.error('W1 idle PostgreSQL connection failed'));

  try {
    await pool.query('SELECT 1');
    const server = createServer({ headersTimeout: 2000, requestTimeout: 5000, keepAliveTimeout: 10000 },
      webhook(pool));
    server.listen(port, host);
    await once(server, 'listening');
    return { server, pool };
  } catch {
    await pool.end();
    throw new Error('could not start the loopback W1 fixture with the restricted ingress login');
  }
}

if (import.meta.main) {
  try {
    if (process.argv.length !== 3) throw new Error('usage: node bench/l01/neutral/node/main.mjs <loopback-http-port>');
    const port = fixturePort(process.argv[2]);
    await startFixture(port);
    console.log(`W1_FIXTURE_LISTENING ${host}:${port}`);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
