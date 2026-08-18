import { defineConfig } from 'astro/config';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  output: 'server',
  server: { port: 4000, host: '0.0.0.0' },
  prefetch: { prefetchAll: true },
  vite: {
    plugins: [tailwindcss()],
    resolve: {
      alias: {
        "@components": "/src/components",
        "@assets": "/src/assets",
        "@pages": "/src/pages",
        "@layouts": "/src/layouts",
        "@styles": "/src/styles",
        "@library": "/src/library",
        "@scripts": "/src/scripts",
      },
    },
  },
});
