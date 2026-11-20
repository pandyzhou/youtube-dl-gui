import { mount, RouterLinkStub } from '@vue/test-utils';
import { createMemoryHistory, createRouter } from 'vue-router';
import { defineComponent, nextTick } from 'vue';
import { describe, expect, it } from 'vitest';
import TheSidebar from '../../src/components/TheSidebar.vue';
import { i18n } from '../../src/i18n';
import { useMediaGroupStore } from '../../src/stores/media/group';
import { useSettingsStore } from '../../src/stores/settings';
import { defaultSettings } from '../../src/tauri/types/config';

function createBlankView() {
  return defineComponent({
    template: '<div />',
  });
}

async function createSidebarRouter(initialPath = '/') {
  const routeNames = ['location', 'authentication', 'subtitles', 'input-filters', 'settings.downloads', 'settings.about'];
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'home', component: createBlankView() },
      ...routeNames.map(name => ({ path: `/${name}`, name, component: createBlankView() })),
    ],
  });
  await router.push(initialPath);
  await router.isReady();
  return router;
}

describe('TheSidebar', () => {
  it('links to every page, with settings opening the downloads tab', async () => {
    const router = await createSidebarRouter();
    const wrapper = mount(TheSidebar, {
      global: {
        plugins: [router, i18n],
        stubs: {
          RouterLink: RouterLinkStub,
        },
      },
    });

    const targets = wrapper.findAllComponents(RouterLinkStub).map(link => link.props('to'));
    expect(targets).toContainEqual({ name: 'home' });
    expect(targets).toContainEqual({ name: 'location' });
    expect(targets).toContainEqual({ name: 'authentication' });
    expect(targets).toContainEqual({ name: 'subtitles' });
    expect(targets).toContainEqual({ name: 'input-filters' });
    expect(targets).toContainEqual({ name: 'settings.downloads' });
  });

  it('marks the current page and shows how many items are queued', async () => {
    const router = await createSidebarRouter('/');
    const groupStore = useMediaGroupStore();
    groupStore.createGroup({
      id: 'g1',
      url: 'https://example.com/video',
      total: 1,
      processed: 1,
      errored: 0,
      isCombined: false,
      audioCodecs: [],
      formats: [],
      filesize: 0,
      items: {
        a: { id: 'a', url: 'https://example.com/video', title: 'A', audioCodecs: [], formats: [], filesize: 0, isLeader: true },
      },
    });

    const wrapper = mount(TheSidebar, {
      global: {
        plugins: [router, i18n],
      },
    });
    await nextTick();

    const current = wrapper.get('a[aria-current="page"]');
    expect(current.text()).toContain('Queue');
    expect(current.text()).toContain('1 in queue');

    await router.push('/subtitles');
    await nextTick();
    expect(wrapper.get('a[aria-current="page"]').text()).toContain('Subtitles');
  });

  it('flags the subtitles page when AI translation is switched on', async () => {
    const router = await createSidebarRouter();
    const settingsStore = useSettingsStore();
    const settings = structuredClone(defaultSettings);
    settings.translation.enabled = true;
    settingsStore.settings = settings;

    const wrapper = mount(TheSidebar, {
      global: {
        plugins: [router, i18n],
      },
    });
    await nextTick();

    const subtitlesLink = wrapper.findAll('a').find(link => link.text().includes('Subtitles'));
    expect(subtitlesLink?.text()).toContain('(on)');
  });
});
