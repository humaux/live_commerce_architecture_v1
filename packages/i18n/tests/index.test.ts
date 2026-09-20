import test from 'node:test';
import assert from 'node:assert/strict';
import { locales, localeFromPath, localizedPath, messages, resolveLocale, translate, type MessageKey } from '../src/index.ts';

test('dictionaries have exact parity and non-empty values', () => {
  const keys = Object.keys(messages.en).sort();
  assert.equal(keys.length, 38);
  for (const locale of locales) assert.deepEqual(Object.keys(messages[locale]).sort(), keys);
  for (const locale of locales) for (const key of keys as MessageKey[]) assert.ok(translate(locale, key));
});

test('locale precedence and language quality mapping', () => {
  const base = { pathname: '/zh-TW/products?x=1#h', defaultLocale: 'en' as const };
  assert.equal(resolveLocale({ ...base, preference: 'en', acceptLanguage: 'en;q=1' }), 'zh-TW');
  assert.equal(resolveLocale({ ...base, pathname: '/products', preference: 'zh-CN' }), 'zh-CN');
  assert.equal(resolveLocale({ ...base, pathname: '/products', preference: 'bad', acceptLanguage: 'zh-Hant-TW;q=0.9,en;q=1' }), 'en');
  assert.equal(resolveLocale({ ...base, pathname: '/products', acceptLanguage: 'zh-Hant;q=0.9,en;q=0' }), 'zh-TW');
  assert.equal(resolveLocale({ ...base, pathname: '/products', acceptLanguage: 'fr;q=1' }), 'en');
  assert.equal(resolveLocale({ ...base, pathname: '/products', acceptLanguage: 'fr;q=no,en;q=0.4' }), 'en');
  assert.equal(resolveLocale({ ...base, pathname: '/products', acceptLanguage: `${'en,'.repeat(40)}zh-CN` }), 'en');
});

test('localized paths preserve deep links and reject dangerous input', () => {
  assert.equal(localizedPath('en', '/products/42?x=1#top'), '/en/products/42?x=1#top');
  assert.equal(localizedPath('zh-CN', '/zh-TW/products/42'), '/zh-CN/products/42');
  assert.equal(localizedPath('en', '/'), '/en/');
  for (const path of ['https://evil.test/x', '//evil.test/x', '/a/../b', '/a/%2e%2e/b', '/a/%252e%252e/b', '/a/%2f/b', '/a/%252f/b', '/a/%', '/a\\b', '/a\u0000b', `/${'x'.repeat(4096)}`]) {
    assert.throws(() => localizedPath('en', path), RangeError);
  }
});

test('path locale only matches a canonical first segment', () => {
  assert.equal(localeFromPath('/en/products'), 'en');
  assert.equal(localeFromPath('/en-US/products'), undefined);
  assert.equal(localeFromPath('/products/en'), undefined);
  assert.equal(localeFromPath('https://example.test/en'), undefined);
});
