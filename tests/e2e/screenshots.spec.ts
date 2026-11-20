import { expect, Page } from '@playwright/test';
import { test } from './utils/fixtures';

// Captures the main screens in both themes so UI changes can be reviewed from
// the CI artifact (e2e-screenshots) without running the app. Nothing here
// asserts on the look itself.
const SHOTS = 'test-results/screenshots';

async function addUrl(page: Page, url: string) {
  await page.fill('input[type="text"]', url);
  await page.getByRole('button', { name: 'Add', exact: true }).click();
}

const pages: [string, string][] = [
  ['settings-downloads', '/settings'],
  ['settings-app', '/settings/app'],
  ['subtitles', '/subtitles'],
  ['location', '/location'],
  ['authentication', '/authentication'],
  ['input-filters', '/input-filters'],
];

for (const scheme of ['dark', 'light'] as const) {
  test.describe(`screenshots (${scheme})`, () => {
    test.use({ colorScheme: scheme, viewport: { width: 1000, height: 900 } });

    test('queue', async ({ page }) => {
      await page.goto('/');
      await page.screenshot({ path: `${SHOTS}/${scheme}-01-queue-empty.png` });

      for (const name of ['a', 'b', 'c', 'd']) {
        await addUrl(page, `https://example.com/video-${name}`);
      }
      await expect(page.locator('article.card')).toHaveCount(4);

      await page.evaluate(async () => {
        // @ts-ignore resolved by the Vite dev server at runtime
        const { useMediaGroupStore } = await import('/src/stores/media/group.ts');
        // @ts-ignore resolved by the Vite dev server at runtime
        const { useMediaStateStore, MediaState } = await import('/src/stores/media/state.ts');
        const ids = Object.keys(useMediaGroupStore().groups);
        const stateStore = useMediaStateStore();
        stateStore.setGroupState(ids[1], MediaState.downloading);
        stateStore.setGroupState(ids[2], MediaState.done);
        stateStore.setGroupState(ids[3], MediaState.error);
      });
      await page.screenshot({ path: `${SHOTS}/${scheme}-02-queue.png` });
    });

    test('pages', async ({ page }) => {
      await page.setViewportSize({ width: 1000, height: 1300 });
      for (const [index, [name, path]] of pages.entries()) {
        await page.goto(path);
        await page.waitForTimeout(300);
        await page.screenshot({ path: `${SHOTS}/${scheme}-${String(index + 3).padStart(2, '0')}-${name}.png` });
      }
    });
  });
}
