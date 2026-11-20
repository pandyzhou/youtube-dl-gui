import { afterEach, describe, expect, it, vi } from 'vitest';
import { getDefaultLocale } from '../../src/i18n.ts';

function withLanguages(languages: string[]) {
  vi.stubGlobal('navigator', { languages, language: languages[0] });
}

describe('Chinese locale detection', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it.each([
    ['zh-CN', 'zh-CN'],
    ['zh-Hans-CN', 'zh-CN'],
    ['zh-SG', 'zh-CN'],
    ['zh', 'zh-CN'],
    ['zh-TW', 'zh-TW'],
    ['zh-HK', 'zh-TW'],
    ['zh-MO', 'zh-TW'],
    ['zh-Hant-HK', 'zh-TW'],
  ])('maps %s to %s', (input, expected) => {
    withLanguages([input]);
    expect(getDefaultLocale()).toBe(expected);
  });

  it('respects language priority order', () => {
    withLanguages(['en-US', 'zh-CN']);
    expect(getDefaultLocale()).toBe('en');
  });
});
