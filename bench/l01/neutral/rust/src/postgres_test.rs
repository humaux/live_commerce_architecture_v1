//! Fresh PG18 conformance for the normal listener. Admin sets up faults, reader observes durable rows.
use super::*;
use http_body_util::{BodyExt, Full, Limited};
use hyper::{Request, Response, body::Incoming, client::conn::http1};
use hyper_util::rt::TokioIo;
use ring::digest;
use serde_json::Value;
use std::{net::SocketAddr, path::Path, time::Duration};
use tokio::{
    net::TcpStream,
    process::Command,
    task::JoinHandle,
    time::{Instant, sleep, timeout},
};
use tokio_postgres::{Client, NoTls};
use uuid::Uuid;

async fn connect(user: &str, password: &str) -> Result<Client> {
    let (client, connection) = pg::config(user, password)?
        .application_name("l01-w1-rust-control")
        .connect(NoTls)
        .await?;
    tokio::spawn(connection);
    Ok(client)
}

async fn oracle(name: &str, args: &[&str], credentials: Option<(&str, &str)>) -> Result<()> {
    let mut cmd = Command::new("node");
    cmd.env_clear()
        .kill_on_drop(true)
        .arg(Path::new(env!("CARGO_MANIFEST_DIR")).join("..").join(name))
        .args(args);
    for var in ["PATH", "HOME", "TMPDIR"] {
        if let Some(v) = env::var_os(var) {
            cmd.env(var, v);
        }
    }
    if let Some((user, password)) = credentials {
        for var in [
            "PGHOST",
            "PGPORT",
            "LC_TEST_DATABASE_ALLOWED",
            "LC_W1_PG_CONTAINER",
            "DOCKER_CONFIG",
            "DOCKER_HOST",
            "DOCKER_CONTEXT",
        ] {
            if let Some(v) = env::var_os(var) {
                cmd.env(var, v);
            }
        }
        cmd.env("PGUSER", user).env("PGPASSWORD", password);
    }
    let status = timeout(Duration::from_secs(35), cmd.status()).await??;
    if !status.success() {
        return Err(format!("{name} failed {status}").into());
    }
    Ok(())
}

struct Probe {
    connection: JoinHandle<std::result::Result<(), hyper::Error>>,
    response: JoinHandle<std::result::Result<Response<Incoming>, hyper::Error>>,
}
impl Probe {
    async fn send(addr: SocketAddr, body: &str) -> Result<Self> {
        let stream = TcpStream::connect(addr).await?;
        let (mut client, connection) = http1::handshake(TokioIo::new(stream)).await?;
        let connection = tokio::spawn(connection);
        let sig = hmac::sign(&HMAC, body.as_bytes());
        let request = Request::post(PATH)
            .header("host", addr.to_string())
            .header("content-type", "application/json")
            .header(
                "x-hub-signature-256",
                format!("sha256={}", hex::encode(sig)),
            )
            .body(Full::new(Bytes::copy_from_slice(body.as_bytes())))?;
        let response = tokio::spawn(async move { client.send_request(request).await });
        Ok(Self {
            connection,
            response,
        })
    }

    async fn expect(self, code: u16) -> Result<()> {
        let r = self.response.await??;
        assert_eq!(r.status(), code);
        let body = Limited::new(r.into_body(), 512).collect().await?.to_bytes();
        assert_eq!(body == "EVENT_RECEIVED", code == 200);
        Ok(())
    }

    async fn disconnect(self) {
        assert!(
            !self.response.is_finished(),
            "received headers before disconnect"
        );
        self.connection.abort();
        assert!(self.connection.await.unwrap_err().is_cancelled());
        assert!(
            self.response.await.unwrap().is_err(),
            "client received a reply"
        );
    }
}

async fn pair(reader: &Client, mid: &str) -> Result<(i64, i64, Option<String>)> {
    let r = reader
        .query_one(
            "SELECT count(*), count(n.inbox_id), min(i.body_sha256)
        FROM l01_w1.inbox i LEFT JOIN l01_w1.intent n ON n.inbox_id = i.id
        WHERE i.external_key=$1",
            &[&mid],
        )
        .await?;
    Ok((r.get(0), r.get(1), r.get(2)))
}

async fn waiting(admin: &Client, event: &str, expected: i64) -> Result<()> {
    let limit = Instant::now() + Duration::from_secs(2);
    loop {
        let n: i64 = admin
            .query_one(
                "SELECT count(*) FROM pg_stat_activity
            WHERE datname=current_database() AND usename='l01_w1_ingress'
            AND application_name='l01-w1-rust' AND wait_event_type='Lock' AND wait_event=$1
            AND ($1 <> 'advisory' OR query='COMMIT')",
                &[&event],
            )
            .await?
            .get(0);
        if n == expected {
            return Ok(());
        }
        assert!(
            Instant::now() < limit,
            "{n}/{expected} PostgreSQL {event} waiters"
        );
        sleep(Duration::from_millis(10)).await;
    }
}

async fn faults(admin: &Client, reader: &Client, addr: SocketAddr, extended: bool) -> Result<()> {
    admin
        .batch_execute(
            "CREATE FUNCTION l01_w1.test_rust_commit() RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN
            IF NEW.external_key='m.rust.fail' THEN RAISE EXCEPTION 'deferred fixture failure';
            ELSIF NEW.external_key IN ('m.rust.held','m.rust.disconnect') THEN
                PERFORM pg_advisory_xact_lock(190118);
            END IF;
            RETURN NEW;
        END $$;
        CREATE CONSTRAINT TRIGGER test_rust_commit AFTER INSERT ON l01_w1.inbox
        DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION l01_w1.test_rust_commit();",
        )
        .await?;
    let corpus: Value = serde_json::from_str(CORPUS)?;
    let sample = corpus["cases"]
        .as_array()
        .unwrap()
        .iter()
        .find(|c| c["expected_admit"] == true)
        .unwrap();
    let body = |mid: &str| {
        sample["raw_body"]
            .as_str()
            .unwrap()
            .replace(sample["event_id"].as_str().unwrap(), mid)
    };
    let absent = (0, 0, None);
    let committed = |raw: &str| {
        (
            1,
            1,
            Some(hex::encode(digest::digest(&digest::SHA256, raw.as_bytes()))),
        )
    };
    Probe::send(addr, &body("m.rust.fail"))
        .await?
        .expect(503)
        .await?;
    assert_eq!(pair(reader, "m.rust.fail").await?, absent);

    for mid in ["m.rust.held", "m.rust.disconnect"] {
        admin
            .query_one("SELECT pg_advisory_lock(190118)", &[])
            .await?;
        let raw = body(mid);
        let first = Probe::send(addr, &raw).await?;
        waiting(admin, "advisory", 1).await?;
        assert_eq!(pair(reader, mid).await?, absent);
        assert!(!first.response.is_finished());
        if mid == "m.rust.held" {
            let replay = Probe::send(addr, &raw).await?;
            let conflict = Probe::send(addr, &raw.replace("fixed-body", "changed-body")).await?;
            waiting(admin, "transactionid", 2).await?;
            assert_eq!(pair(reader, mid).await?, absent);
            assert!(
                [&first, &replay, &conflict]
                    .iter()
                    .all(|p| !p.response.is_finished())
            );
            assert!(
                admin
                    .query_one("SELECT pg_advisory_unlock(190118)", &[])
                    .await?
                    .get::<_, bool>(0)
            );
            first.expect(200).await?;
            replay.expect(200).await?;
            conflict.expect(409).await?;
        } else {
            first.disconnect().await;
            assert!(
                admin
                    .query_one("SELECT pg_advisory_unlock(190118)", &[])
                    .await?
                    .get::<_, bool>(0)
            );
            let limit = Instant::now() + Duration::from_secs(2);
            while pair(reader, mid).await? != committed(&raw) {
                assert!(
                    Instant::now() < limit,
                    "original delivery did not commit after disconnect"
                );
                sleep(Duration::from_millis(10)).await;
            }
            Probe::send(addr, &raw).await?.expect(200).await?;
        }
        assert_eq!(pair(reader, mid).await?, committed(&raw));
    }

    if extended {
        for asset in ["1000000001901103", "1000000001901104"] {
            let mid = "m.rust.staged";
            let raw = body(mid).replace(corpus["authoritative_asset_id"].as_str().unwrap(), asset);
            let sql = "SELECT json_build_array(id,key_id,nonce,ciphertext)::text
                FROM l01_w1.quarantine WHERE asset_id=$1 AND external_key=$2";
            Probe::send(addr, &raw).await?.expect(200).await?;
            let original: String = reader.query_one(sql, &[&asset, &mid]).await?.get(0);
            Probe::send(addr, &raw).await?.expect(200).await?;
            assert_eq!(
                reader
                    .query_one(sql, &[&asset, &mid])
                    .await?
                    .get::<_, String>(0),
                original
            );
            assert_eq!(pair(reader, mid).await?, absent);
        }
    }
    Ok(())
}

#[tokio::test]
#[ignore = "select base or extended on its own fresh disposable PostgreSQL 18 cluster"]
async fn fresh_pg18() -> Result<()> {
    timeout(Duration::from_secs(95), fresh()).await?
}

async fn fresh() -> Result<()> {
    let mode = env::var("LC_W1_RUST_MODE")?;
    assert!(matches!(mode.as_str(), "base" | "extended"));
    assert_eq!(env::var("PGUSER")?, "postgres");
    let admin_password = env::var("PGPASSWORD")?;
    let admin = connect("postgres", &admin_password).await?;
    let fresh: bool = admin.query_one("SELECT current_setting('server_version_num')::int BETWEEN 180000 AND 189999
        AND current_database()='lc_neutral_w1_test' AND session_user='postgres' AND current_user=session_user
        AND to_regnamespace('l01_w1') IS NULL AND NOT EXISTS (
            SELECT FROM pg_roles WHERE rolname IN ('l01_w1_ingress','l01_w1_reader','l01_w1_owner'))",
        &[]).await?.get(0);
    assert!(
        fresh,
        "requires separate fresh disposable PG18 cluster per mode"
    );
    oracle(
        "pg/check.mjs",
        &["setup"],
        Some(("postgres", &admin_password)),
    )
    .await?;
    let ingress_password = Uuid::new_v4().simple().to_string();
    let reader_password = Uuid::new_v4().simple().to_string();
    admin
        .batch_execute(&format!(
            "ALTER ROLE l01_w1_ingress LOGIN PASSWORD '{ingress_password}';
        ALTER ROLE l01_w1_reader LOGIN PASSWORD '{reader_password}';"
        ))
        .await?;
    let reader = connect("l01_w1_reader", &reader_password).await?;
    let pool = pg::pool(&ingress_password).await?;
    let ingress = pool.get().await?;
    for e in [
        ingress
            .query("SELECT * FROM l01_w1.inbox LIMIT 0", &[])
            .await
            .unwrap_err(),
        reader
            .query("INSERT INTO l01_w1.inbox DEFAULT VALUES", &[])
            .await
            .unwrap_err(),
        reader
            .query("SELECT l01_w1.admit(NULL,NULL,NULL,NULL,NULL)", &[])
            .await
            .unwrap_err(),
    ] {
        assert_eq!(
            e.code(),
            Some(&tokio_postgres::error::SqlState::INSUFFICIENT_PRIVILEGE)
        );
    }
    drop(ingress);
    let listener = TcpListener::bind(("127.0.0.1", 0)).await?;
    let addr = listener.local_addr()?;
    let server = tokio::spawn(serve(listener, pool));
    let port = addr.port().to_string();
    let extended = mode == "extended";
    let mut args = vec![port.as_str()];
    if extended {
        args.push("--extended");
    }
    oracle("http/check.mjs", &args, None).await?;
    oracle(
        "pg/check.mjs",
        &[if extended {
            "readback-extended"
        } else {
            "readback"
        }],
        Some(("l01_w1_reader", &reader_password)),
    )
    .await?;
    faults(&admin, &reader, addr, extended).await?;
    server.abort();
    assert!(server.await.unwrap_err().is_cancelled());
    Ok(())
}
