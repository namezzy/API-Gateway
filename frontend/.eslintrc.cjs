module.exports = {
  root: true,
  parser: '@typescript-eslint/parser',
  plugins: ['@typescript-eslint','react-refresh'],
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended'
  ],
  env: { browser: true, es2022: true, node: true },
  rules: {
    '@typescript-eslint/no-explicit-any': 'off',
    'react-refresh/only-export-components': 'warn'
  }
};
