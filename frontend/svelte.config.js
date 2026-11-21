import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

export default {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter(),
    alias: {
      '@components': 'src/lib/components',
      '@assets': 'src/lib/assets',
      '@library': 'src/lib',
      '@styles': 'src/styles',
    },
  },
};
