const { defineConfig } = require('eslint/config');
const expoConfig = require('eslint-config-expo/flat');

module.exports = defineConfig([
  expoConfig,
  { ignores: ['dist/*', '.expo/*', 'src/uniwind-types.d.ts'] },
  {
    files: ['**/*.ts', '**/*.tsx'],
    rules: {
      // The contract types live outside the app (packages/api-types) and are resolved only by
      // TypeScript (tsconfig `paths`): a value import would reach Metro and break the bundle.
      '@typescript-eslint/no-restricted-imports': [
        'error',
        { paths: [{ name: '@lenapay/api-types', message: 'Import contract types with `import type` only.', allowTypeImports: true }] },
      ],
    },
  },
]);
