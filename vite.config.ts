import { defineConfig, UserConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import tailwindcss from '@tailwindcss/vite';

const host = process.env.TAURI_DEV_HOST;

const isE2E = process.env.E2E === 'true';
const isDev = process.env.DEV === 'true';

export default defineConfig((): UserConfig => ({
  plugins: [
    vue(),
    tailwindcss(),
  ],
  define: {
    __DEV__: isDev,
    __E2E__: isE2E,
    // build-windows sets APP_VERSION (e.g. 3.2.1-zh.11); package.json only has the base version.
    __APP_VERSION__: JSON.stringify(process.env.APP_VERSION || process.env.npm_package_version),
  },
  clearScreen: false,
  build: {
    chunkSizeWarningLimit: 1000,
  },
  server: {
    port: 1420,
    strictPort: true,
    host: host || false,
    hmr: host
      ? {
          protocol: 'ws',
          host,
          port: 1421,
        }
      : undefined,
    watch: {
      ignored: ['**/src-tauri/**', '!**/src-tauri/tauri.conf.json', '**/coverage/**', '**/manifest/**', '**/test-results/**'],
    },
  },
}));
