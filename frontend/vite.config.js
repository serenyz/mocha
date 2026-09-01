import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const proxyTarget = process.env.VITE_PROXY_TARGET || 'http://127.0.0.1:6666';

export default defineConfig({
  plugins: [react()],
  build: {
    target: 'es2022',
    cssTarget: 'chrome120',
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('/node_modules/country-region-data/')) return 'region-data';
          return undefined;
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: proxyTarget,
        changeOrigin: true,
        ws: true,
      },
      '/static': {
        target: proxyTarget,
        changeOrigin: true,
      },
    },
  },
});
