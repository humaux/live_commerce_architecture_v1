//! Direct controls at the same HTTP and commit boundaries as the loopback listener.
use super::*;
use axum::{
    body::{Body, to_bytes},
    http::Request,
};
use serde_json::Value;
use std::{
    sync::{
        Arc,
        atomic::{AtomicUsize, Ordering},
    },
    time::Duration,
};
use tower::ServiceExt;

fn sample() -> String {
    let corpus: Value = serde_json::from_str(CORPUS).unwrap();
    corpus["cases"]
        .as_array()
        .unwrap()
        .iter()
        .find(|c| c["expected_admit"] == true)
        .unwrap()["raw_body"]
        .as_str()
        .unwrap()
        .into()
}
fn signed(raw: &[u8]) -> String {
    format!("sha256={}", hex::encode(hmac::sign(&HMAC, raw).as_ref()))
}
fn request(raw: Vec<u8>, headers: &[String]) -> Request<Body> {
    let mut r = Request::post(PATH);
    for h in headers {
        r = r.header("x-hub-signature-256", h);
    }
    r.body(Body::from(raw)).unwrap()
}

#[tokio::test]
async fn provisional_admission_cannot_ack_until_commit() {
    let gate = Arc::new(tokio::sync::Notify::new());
    let entered = Arc::new(tokio::sync::Notify::new());
    let release = gate.clone();
    let start = entered.clone();
    let service = app(move |_, _, _| {
        let gate = gate.clone();
        let entered = entered.clone();
        async move {
            entered.notify_one();
            pg::committed(Outcome::Ack, async move {
                gate.notified().await;
                Ok::<(), Failure>(())
            })
            .await
        }
    });
    let raw = sample().into_bytes();
    let sig = signed(&raw);
    let mut response = tokio::spawn(service.oneshot(request(raw, &[sig])));
    tokio::time::timeout(Duration::from_secs(1), start.notified())
        .await
        .expect("provisional admission reached");
    assert!(
        tokio::time::timeout(Duration::from_millis(30), &mut response)
            .await
            .is_err(),
        "early ACK before COMMIT"
    );
    release.notify_one();
    let res = response.await.unwrap().unwrap();
    assert_eq!(res.status(), 200);
    assert_eq!(
        to_bytes(res.into_body(), 512).await.unwrap(),
        "EVENT_RECEIVED"
    );
}

#[tokio::test]
async fn failed_commit_rejects_every_exact_repeat() {
    let calls = Arc::new(AtomicUsize::new(0));
    let n = calls.clone();
    let svc = app(move |_, _, _| {
        n.fetch_add(1, Ordering::SeqCst);
        pg::committed(Outcome::Ack, async {
            Err::<(), Failure>("commit failed".into())
        })
    });
    for _ in 0..2 {
        let raw = sample().into_bytes();
        let sig = signed(&raw);
        assert_eq!(
            svc.clone()
                .oneshot(request(raw, &[sig]))
                .await
                .unwrap()
                .status(),
            503
        );
    }
    assert_eq!(calls.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn rejects_without_admission() {
    let svc = app(|_, _, _| async { panic!("unexpected persistence call") });
    let text = sample();
    let raw = text.as_bytes();
    let sig = signed(raw);
    for (bytes, headers, expected) in [
        (raw.to_vec(), vec![], 403),
        (raw.to_vec(), vec![sig.clone(), sig.clone()], 403),
        (b"{".to_vec(), vec![sig.clone()], 403),
        (
            raw.to_vec(),
            vec![format!("sha256={}", sig[7..].to_uppercase())],
            403,
        ),
        (vec![b' '; (1 << 20) + 1], vec![], 413),
    ] {
        assert_eq!(
            svc.clone()
                .oneshot(request(bytes, &headers))
                .await
                .unwrap()
                .status(),
            expected
        );
    }
    let mut invalid_utf8 = raw.to_vec();
    invalid_utf8.splice(2..2, [0xfe]);
    for bytes in [
        invalid_utf8,
        [raw, b"{}"].concat(),
        text.replace("fixed-body", r"\ud800").into_bytes(),
        text.replace(r#""text":"#, r#""t\u0065xt":null,"text":"#)
            .into_bytes(),
        text.replace(r#""recipient":{"id":""#, r#""recipient":{"id":"9"#)
            .into_bytes(),
        text.replace(r#""mid":""#, &format!(r#""mid":"{}"#, "é".repeat(513)))
            .into_bytes(),
        text.replace(r#""object":"page""#, r#""Object":"page""#)
            .into_bytes(),
        text.replace(r#""mid":""#, r#""mid":"\u0000"#).into_bytes(),
    ] {
        let sig = signed(&bytes);
        assert_eq!(
            svc.clone()
                .oneshot(request(bytes, &[sig]))
                .await
                .unwrap()
                .status(),
            400
        );
    }
}

#[test]
fn fresh_quarantine_attempts() {
    let a = pg::seal(b"body", "123", "mid", "digest").unwrap();
    let b = pg::seal(b"body", "123", "mid", "digest").unwrap();
    assert_ne!(a.0, b.0);
    assert_ne!(a.1, b.1);
    assert_ne!(a.2, b.2);
    assert_eq!(a.2.len(), 4 + 16);
}

#[tokio::test]
async fn frozen_wire_inputs_reach_admission_only_when_authenticated() {
    let corpus: Value = serde_json::from_str(CORPUS).unwrap();
    let calls = Arc::new(AtomicUsize::new(0));
    for case in corpus["cases"].as_array().unwrap() {
        let accepted = case["expected_admit"].as_bool().unwrap();
        let digest = case["body_sha256"].as_str().unwrap().to_owned();
        let expected_mid = case["event_id"].as_str().unwrap().to_owned();
        let expected_asset = corpus["authoritative_asset_id"]
            .as_str()
            .unwrap()
            .to_owned();
        let n = calls.clone();
        let service = app(move |bytes, asset, mid| {
            assert!(accepted);
            assert_eq!(asset, expected_asset);
            assert_eq!(mid, expected_mid);
            assert_eq!(
                hex::encode(ring::digest::digest(&ring::digest::SHA256, &bytes)),
                digest
            );
            n.fetch_add(1, Ordering::SeqCst);
            async { Ok(Outcome::Ack) }
        });
        let signature = case["signature_header"].as_str().unwrap().to_owned();
        let raw = case["raw_body"].as_str().unwrap().as_bytes().to_vec();
        let mut request = request(raw, &[signature]);
        request.headers_mut().insert(
            "x-tenant-id",
            case["untrusted_tenant_header"]
                .as_str()
                .unwrap()
                .parse()
                .unwrap(),
        );
        let response = service.oneshot(request).await.unwrap();
        assert_eq!(response.status(), if accepted { 200 } else { 403 });
    }
    assert_eq!(calls.load(Ordering::SeqCst), 80);
}
