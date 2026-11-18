//! Trusted fixture connection pool and admission transaction. Only the frozen SQL function resolves scope.
use super::{CONFIG, Failure, Outcome, Result, port};
use axum::body::Bytes;
use deadpool_postgres::{Hook, HookError, Manager, Pool, Runtime};
use ring::{
    aead, digest,
    rand::{SecureRandom, SystemRandom},
};
use std::{env, future::Future, time::Duration};
use tokio_postgres::{Config, IsolationLevel, NoTls};
use uuid::Uuid;

const KEY_ID: &str = "neutral_w1_fixture";
const IDENTITY: &str = "SELECT session_user = 'l01_w1_ingress' AND current_user = session_user
    AND current_database() = 'lc_neutral_w1_test'
    AND current_setting('server_version_num')::int BETWEEN 180000 AND 189999
    AND NOT (rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole)
    FROM pg_roles WHERE rolname = session_user";

pub(super) fn config(user: &str, password: &str) -> Result<Config> {
    if env::var("LC_TEST_DATABASE_ALLOWED").as_deref() != Ok("1")
        || env::var("PGHOST").as_deref() != Ok("127.0.0.1")
        || ["PGHOSTADDR", "PGSERVICE", "PGOPTIONS"]
            .iter()
            .any(|k| env::var_os(k).is_some_and(|v| !v.is_empty()))
    {
        return Err("requires an opted-in loopback W1 fixture without routing overrides".into());
    }
    let mut config = Config::new();
    config
        .host("127.0.0.1")
        .port(port(&env::var("PGPORT")?)?)
        .dbname("lc_neutral_w1_test")
        .user(user)
        .password(password)
        .application_name("l01-w1-rust")
        .options("-c statement_timeout=4500")
        .connect_timeout(Duration::from_secs(2));
    Ok(config)
}

pub async fn pool(password: &str) -> Result<Pool> {
    let config = config("l01_w1_ingress", password)?;
    let pool = Pool::builder(Manager::new(config, NoTls))
        .runtime(Runtime::Tokio1)
        .max_size(8)
        .wait_timeout(Some(Duration::from_secs(2)))
        .create_timeout(Some(Duration::from_secs(3)))
        .post_create(Hook::async_fn(|c, _| {
            Box::pin(async move {
                let row = c
                    .query_one(IDENTITY, &[])
                    .await
                    .map_err(HookError::Backend)?;
                if !row.get::<_, bool>(0) {
                    return Err(HookError::message("invalid W1 ingress identity"));
                }
                Ok(())
            })
        }))
        .build()?;
    drop(pool.get().await?);
    Ok(pool)
}

fn sha(bytes: &[u8]) -> String {
    hex::encode(digest::digest(&digest::SHA256, bytes))
}

pub(super) fn seal(
    raw: &[u8],
    asset: &str,
    mid: &str,
    hash: &str,
) -> Result<(Uuid, [u8; 12], Vec<u8>)> {
    let id = Uuid::new_v4();
    let mut nonce = [0; 12];
    SystemRandom::new()
        .fill(&mut nonce)
        .map_err(|_| "random nonce unavailable")?;
    let app = CONFIG.app_id.as_str();
    let object = CONFIG.object.as_str();
    let event_key = sha(&serde_json::to_vec(&[app, object, asset, mid])?);
    let aad = serde_json::to_vec(&(
        "livecommerce/meta-payload/v1",
        "quarantine",
        id.to_string(),
        app,
        object,
        "",
        event_key,
        hash,
        "",
        "",
        "",
        0,
        KEY_ID,
    ))?;
    let key = aead::UnboundKey::new(&aead::AES_256_GCM, &[0x31; 32])
        .map_err(|_| "invalid fixture key")?;
    let mut ciphertext = raw.to_vec();
    aead::LessSafeKey::new(key)
        .seal_in_place_append_tag(
            aead::Nonce::assume_unique_for_key(nonce),
            aead::Aad::from(aad),
            &mut ciphertext,
        )
        .map_err(|_| "cannot seal W1 envelope")?;
    Ok((id, nonce, ciphertext))
}

pub(super) async fn committed<E: Into<Failure>>(
    outcome: Outcome,
    commit: impl Future<Output = std::result::Result<(), E>>,
) -> Result<Outcome> {
    commit.await.map_err(Into::into)?;
    Ok(outcome)
}

pub async fn admit(pool: Pool, raw: Bytes, asset: String, mid: String) -> Result<Outcome> {
    let mut client = pool.get().await?;
    let tx = client
        .build_transaction()
        .isolation_level(IsolationLevel::ReadCommitted)
        .start()
        .await?;
    let hash = sha(&raw);
    let app = &CONFIG.app_id;
    let object = &CONFIG.object;
    let args: &[&(dyn tokio_postgres::types::ToSql + Sync)] = &[app, object, &asset, &mid, &hash];
    let row = tx
        .query_one("SELECT l01_w1.admit($1,$2,$3,$4,$5)", args)
        .await?;
    let outcome = match row.try_get::<_, &str>(0)? {
        "needs_quarantine" => {
            let (id, nonce, encrypted) = seal(&raw, &asset, &mid, &hash)?;
            let row = tx
                .query_one(
                    "SELECT l01_w1.admit($1,$2,$3,$4,$5,$6,$7,$8,$9)",
                    &[
                        app,
                        object,
                        &asset,
                        &mid,
                        &hash,
                        &id,
                        &KEY_ID,
                        &&nonce[..],
                        &encrypted,
                    ],
                )
                .await?;
            if row.try_get::<_, &str>(0)? != "quarantined" {
                return Err("unexpected sealed admission".into());
            }
            Outcome::Ack
        }
        "routed" | "replay" => Outcome::Ack,
        "conflict" => Outcome::Conflict,
        _ => return Err("unexpected admission".into()),
    };
    committed(outcome, tx.commit()).await
}
