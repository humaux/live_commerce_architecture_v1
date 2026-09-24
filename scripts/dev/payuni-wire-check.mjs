// Independent Node/OpenSSL check of the official vector and synthetic Go forms.
// Public example keys only: never pass merchant credentials to this test helper.
import crypto from 'node:crypto';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const key = '12345678901234567890123456789012';
const iv = Buffer.from('1234567890123456');
const plain = 'MerID=AAA&MerTradeNO=BBB&Prod=%E5%95%86%E5%93%81%E8%AA%AA%E6%98%8E';
const golden = '47396636346f66735853533167396942344f587a3775696b34732b596e70452b675270564f73536b7753446c6a4d77526d4e374256514173672b6c78616d4533504d475152642b362f4530626f446e4f6356533969756c743a3a3a4b5961342f4635456965743069385a784b6277704a413d3d';
const digest = 'E97180D78C8378D64A188D292938B9D2717034F292B626019B01DF160AEFC0B7';
function encrypt(value) {
  const c = crypto.createCipheriv('aes-256-gcm', key, iv);
  const body = Buffer.concat([c.update(value, 'utf8'), c.final()]);
  return Buffer.from(`${body.toString('base64')}:::${c.getAuthTag().toString('base64')}`).toString('hex');
}
function hash(value) { return crypto.createHash('sha256').update(key + value + iv.toString()).digest('hex').toUpperCase(); }
function decrypt(value) {
  const [body, tag] = Buffer.from(value, 'hex').toString().split(':::');
  const d = crypto.createDecipheriv('aes-256-gcm', key, iv);
  d.setAuthTag(Buffer.from(tag, 'base64'));
  return Buffer.concat([d.update(Buffer.from(body, 'base64')), d.final()]).toString();
}
assert.equal(encrypt(plain), golden);
assert.equal(hash(golden), digest);
assert.equal(decrypt(golden), plain);
if (process.argv[2] === '--form') {
  const form = JSON.parse(readFileSync(0, 'utf8'));
  assert.equal(hash(form.EncryptInfo[0]), form.HashInfo[0]);
  const decrypted = decrypt(form.EncryptInfo[0]);
  assert.equal(encrypt(decrypted), form.EncryptInfo[0]);
  process.stdout.write(JSON.stringify(Object.fromEntries(new URLSearchParams(decrypted))));
} else {
  assert.equal(process.argv.length, 2);
  console.log('PASS: official PAYUNi page312 crypto vector via independent Node/OpenSSL');
}
