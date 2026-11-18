//! Signed W1 input for the loopback handler. Reject duplicate names even in ignored fields.
use axum::http::HeaderMap;
use ring::hmac;
use serde::de::{Deserialize, Deserializer, Error, MapAccess, SeqAccess, Visitor};
use serde_json::Value;
use std::fmt;

pub fn authenticated(headers: &HeaderMap, key: &hmac::Key, bytes: &[u8]) -> bool {
    let mut headers = headers.get_all("x-hub-signature-256").iter();
    let Some(mac) = headers
        .next()
        .and_then(|v| v.as_bytes().strip_prefix(b"sha256="))
    else {
        return false;
    };
    let mut signature = [0; 32];
    headers.next().is_none()
        && !mac.iter().any(u8::is_ascii_uppercase)
        && hex::decode_to_slice(mac, &mut signature).is_ok()
        && hmac::verify(key, bytes, &signature).is_ok()
}

// Only strings, arrays and object members take part in delivery routing.
struct Field(Value);
impl<'de> Deserialize<'de> for Field {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        d.deserialize_any(Fields)
    }
}

struct Fields;
impl<'de> Visitor<'de> for Fields {
    type Value = Field;
    fn expecting(&self, f: &mut fmt::Formatter) -> fmt::Result {
        f.write_str("a JSON value with unique object member names")
    }
    fn visit_map<A: MapAccess<'de>>(self, mut a: A) -> Result<Field, A::Error> {
        let mut object = serde_json::Map::new();
        while let Some(key) = a.next_key::<String>()? {
            if object.contains_key(&key) {
                return Err(A::Error::custom("duplicate member"));
            }
            object.insert(key, a.next_value::<Field>()?.0);
        }
        Ok(Field(Value::Object(object)))
    }
    fn visit_seq<A: SeqAccess<'de>>(self, mut a: A) -> Result<Field, A::Error> {
        let mut values = Vec::new();
        while let Some(Field(v)) = a.next_element()? {
            values.push(v);
        }
        Ok(Field(Value::Array(values)))
    }
    fn visit_str<E>(self, v: &str) -> Result<Field, E> {
        Ok(Field(v.into()))
    }
    fn visit_i64<E>(self, _: i64) -> Result<Field, E> {
        Ok(Field(Value::Null))
    }
    fn visit_u64<E>(self, _: u64) -> Result<Field, E> {
        Ok(Field(Value::Null))
    }
    fn visit_f64<E>(self, _: f64) -> Result<Field, E> {
        Ok(Field(Value::Null))
    }
    fn visit_bool<E>(self, _: bool) -> Result<Field, E> {
        Ok(Field(Value::Null))
    }
    fn visit_unit<E>(self) -> Result<Field, E> {
        Ok(Field(Value::Null))
    }
}

pub fn delivery(raw: &[u8], object: &str) -> Option<(String, String)> {
    let Field(v) = serde_json::from_slice(raw).ok()?;
    let [entry] = v["entry"].as_array()?.as_slice() else {
        return None;
    };
    let [msg] = entry["messaging"].as_array()?.as_slice() else {
        return None;
    };
    let asset = entry["id"].as_str()?;
    let mid = msg["message"]["mid"].as_str()?;
    (v["object"].as_str() == Some(object)
        && (1..=40).contains(&asset.len())
        && asset.bytes().all(|b| b.is_ascii_digit())
        && msg["recipient"]["id"].as_str() == Some(asset)
        && (1..=1024).contains(&mid.len())
        && !mid.contains('\0'))
    .then(|| (asset.to_owned(), mid.to_owned()))
}
