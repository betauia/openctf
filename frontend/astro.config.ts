import { defineConfig } from 'astro/config';

export default defineConfig({
  output: 'server',
  server: { port: 4000, host: '0.0.0.0' },
  vite: {
    resolve: {
      alias: {
        "@components": "/src/components",
        "@assets": "/src/assets",
        "@pages": "/src/pages",
        "@layouts": "/src/layouts",
        "@styles": "/src/styles",
        "@lib": "/src/lib",
        "@scripts": "/src/scripts",
      },
    },
  },
});
