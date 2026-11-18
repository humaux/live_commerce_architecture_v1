//! W1 fixture CLI and HTTP boundary. Uses the checked-in corpus and restricted PG adapter only.
mod input;
mod pg;

use axum::{
    Router,
    body::Bytes,
    extract::DefaultBodyLimit,
    http::{HeaderMap, StatusCode},
    routing::post,
};
use ring::hmac;
use serde::Deserialize;
use std::{env, error::Error, future::Future, sync::LazyLock};
use tokio::net::TcpListener;

type Failure = Box<dyn Error + Send + Sync>;
type Result<T> = std::result::Result<T, Failure>;
const PATH: &str = "/__l01/w1/webhook";
const CORPUS: &str = include_str!("../../w1_corpus.json");

#[derive(Deserialize)]
struct Config {
    app_id: String,
    object: String,
    hmac_secret: String,
    synthetic_only: bool,
}
static CONFIG: LazyLock<Config> = LazyLock::new(|| {
    let c: Config = serde_json::from_str(CORPUS).expect("checked-in W1 corpus");
    assert!(c.synthetic_only);
    c
});
static HMAC: LazyLock<hmac::Key> =
    LazyLock::new(|| hmac::Key::new(hmac::HMAC_SHA256, CONFIG.hmac_secret.as_bytes()));

#[derive(Clone, Copy)]
enum Outcome {
    Ack,
    Conflict,
}

fn app<F, Fut>(admit: F) -> Router
where
    F: Fn(Bytes, String, String) -> Fut + Clone + Send + Sync + 'static,
    Fut: Future<Output = Result<Outcome>> + Send,
{
    Router::new()
        .route(
            PATH,
            post(move |headers: HeaderMap, raw: Bytes| {
                let admit = admit.clone();
                async move {
                    if !input::authenticated(&headers, &HMAC, &raw) {
                        return (StatusCode::FORBIDDEN, "");
                    }
                    let Some((asset, mid)) = input::delivery(&raw, &CONFIG.object) else {
                        return (StatusCode::BAD_REQUEST, "");
                    };
                    match admit(raw, asset, mid).await {
                        Ok(Outcome::Ack) => (StatusCode::OK, "EVENT_RECEIVED"),
                        Ok(Outcome::Conflict) => (StatusCode::CONFLICT, ""),
                        Err(_) => (StatusCode::SERVICE_UNAVAILABLE, ""),
                    }
                }
            }),
        )
        .layer(DefaultBodyLimit::max(1 << 20))
}

fn port(value: &str) -> Result<u16> {
    let port: u16 = value.parse()?;
    if port == 0 || value != port.to_string() {
        return Err("expected explicit port 1..65535".into());
    }
    Ok(port)
}

#[tokio::main]
async fn main() -> Result<()> {
    let args: Vec<_> = env::args().skip(1).collect();
    let [arg] = &args[..] else {
        return Err("usage: w1-rust <loopback-http-port>".into());
    };
    let port = port(arg)?;
    let pool = pg::pool(&env::var("PGPASSWORD")?).await?;
    let listener = TcpListener::bind(("127.0.0.1", port)).await?;
    println!("W1_FIXTURE_LISTENING {}", listener.local_addr()?);
    serve(listener, pool).await?;
    Ok(())
}

#[cfg(test)]
mod tests;

async fn serve(listener: TcpListener, pool: deadpool_postgres::Pool) -> std::io::Result<()> {
    axum::serve(
        listener,
        app(move |raw, asset, mid| pg::admit(pool.clone(), raw, asset, mid)),
    )
    .await
}

#[cfg(test)]
mod postgres_test;
