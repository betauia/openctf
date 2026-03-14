// @ts-check
import { defineConfig } from 'astro/config';

// https://astro.build/config
export default defineConfig({
    server: { port: 4000, host: '0.0.0.0'},
    vite: {
        resolve: {
            alias: {
                "@components": "/src/components",
                "@assets": "/src/assets",
                "@pages": "/src/pages",
            },
        },
    },
});
