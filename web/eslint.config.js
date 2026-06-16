// ESLint flat config for the Tala WTE web console (Svelte 5 + TypeScript).
// Run with `pnpm lint` (or `make lint` from the repo root). Formatting is owned
// by Prettier; eslint-config-prettier disables stylistic rules so the two never
// fight. A few project coding standards are encoded as lint errors.
import js from '@eslint/js';
import ts from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import prettier from 'eslint-config-prettier';
import globals from 'globals';

// Project rule (engineering-output-standards): no en/em dashes anywhere in
// shipped text. These live in the BMP, so a plain esquery value regex matches
// them reliably in string and template literals (and Svelte <script> blocks).
const noDashes = [
	{
		selector: 'Literal[value=/[\\u2013\\u2014]/]',
		message: 'No en/em dashes - use an ASCII hyphen "-".'
	},
	{
		selector: 'TemplateElement[value.raw=/[\\u2013\\u2014]/]',
		message: 'No en/em dashes - use an ASCII hyphen "-".'
	}
];

export default ts.config(
	{
		// Build output and generated SvelteKit internals are not ours to lint.
		ignores: [
			'build/',
			'.svelte-kit/',
			'dist/',
			'node_modules/',
			'eslint.config.js',
			'svelte.config.js',
			'vite.config.ts'
		]
	},
	js.configs.recommended,
	...ts.configs.recommended,
	...svelte.configs.recommended,
	prettier,
	...svelte.configs.prettier,
	{
		languageOptions: {
			globals: { ...globals.browser, ...globals.node }
		}
	},
	{
		// Svelte files are parsed by svelte-eslint-parser; hand the embedded
		// <script lang="ts"> to the TypeScript parser.
		files: ['**/*.svelte', '**/*.svelte.ts'],
		languageOptions: {
			parserOptions: { parser: ts.parser }
		}
	},
	{
		rules: {
			// catch (e: any) is the established error-handling shape here and some
			// sites read provider-specific fields off the error; flag it as tracked
			// tech-debt (warn) rather than blocking the release on a broad refactor.
			'@typescript-eslint/no-explicit-any': 'warn',
			// Allow a leading-underscore name to mark a deliberately unused binding
			// (e.g. `const _ = lines.length` to register a reactive dependency).
			'@typescript-eslint/no-unused-vars': [
				'error',
				{
					argsIgnorePattern: '^_',
					varsIgnorePattern: '^_',
					caughtErrorsIgnorePattern: '^_'
				}
			],
			// The codebase has no console calls; keep it that way.
			'no-console': 'error',
			'no-debugger': 'error',
			'no-restricted-syntax': ['error', ...noDashes],
			// Only meaningful for apps that resolve() a base path or i18n routes;
			// Tala WTE is served from root, so plain hrefs/goto() are correct.
			'svelte/no-navigation-without-resolve': 'off',
			// Keyed {#each} is a good default but unkeyed lists are not bugs here;
			// surface as tech-debt rather than blocking the release.
			'svelte/require-each-key': 'warn',
			// {@html} is used deliberately for trusted static SVG icon strings, and
			// xterm/log panes need direct DOM access; flag for review, don't block.
			'svelte/no-at-html-tags': 'warn',
			'svelte/no-dom-manipulating': 'warn',
			'svelte/prefer-svelte-reactivity': 'warn'
		}
	}
);
