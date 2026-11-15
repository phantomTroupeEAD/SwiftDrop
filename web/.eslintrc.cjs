module.exports = {
  root: true,
  env: { browser: true, es2020: true },
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:react/recommended',
    'plugin:react/jsx-runtime',
  ],
  ignorePatterns: [
    'dist',
    'node_modules',
    '*.config.js',
    '*.config.cjs',
    'scripts',
    'src/shared/components/ui/**',
    // Tracked legacy pre-shadcn prototype screens (to be migrated in post-foundation phase)
    'src/features/dashboards/AdminDashboard.tsx',
    'src/features/dashboards/DashboardLayout.tsx',
    'src/features/dashboards/MerchantDashboard.tsx',
  ],
  parser: '@typescript-eslint/parser',
  settings: {
    react: {
      version: 'detect',
    },
  },
  rules: {
    '@typescript-eslint/no-explicit-any': 'warn',
    '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
    'react/prop-types': 'off',
    'no-empty': ['error', { allowEmptyCatch: true }],
    'no-restricted-syntax': [
      'error',
      {
        selector: 'JSXElement > JSXOpeningElement > JSXIdentifier[name=/^(button|input|select|textarea|table|dialog)$/]',
        message: 'Direct use of intrinsic HTML form, table, or dialog elements (<button>, <input>, <select>, <textarea>, <table>, <dialog>) is prohibited in feature code. Compose exclusively from @/shared/components/ui/ primitives (AGENTS.md Rule 3).',
      },
    ],
    'no-restricted-imports': [
      'error',
      {
        patterns: [
          {
            group: ['@radix-ui/*'],
            message: 'Direct imports from @radix-ui/* are prohibited in feature code. Compose exclusively from @/shared/components/ui/ primitives (AGENTS.md Rule 2).',
          },
        ],
      },
    ],
  },
};
