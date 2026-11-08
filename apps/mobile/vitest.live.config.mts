import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';

/**
 * `pnpm test:live`: the app's live client against a running API (docker compose stack, seeded).
 * Not part of `pnpm test`: it needs the stack and changes its data (creates and approves a payout).
 * Base URL from LIVE_API_URL (default http://localhost:18080).
 */
export default defineConfig({
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.live.ts'],
    setupFiles: ['./vitest.setup.ts'],
    testTimeout: 30_000,
    fileParallelism: false,
  },
});
