// @ts-check
// JetStore's documentation site. Served by the apiserver at /docs/ on an
// installation, from the directory the UI image copies dist/ to.
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightLinksValidator from 'starlight-links-validator';

export default defineConfig({
	base: '/docs',
	// Every page is a directory with an index.html (/docs/user/), and every
	// internal link ends with a slash, which the links validator enforces.
	trailingSlash: 'always',
	build: { format: 'directory' },
	outDir: './dist',
	integrations: [
		starlight({
			title: 'JetStore',
			description: 'JetStore documentation: the User, Developer\'s and Installation Guides.',
			// Fails the build on a broken internal link or anchor, or a relative link.
			plugins: [starlightLinksValidator()],
			// Gives each folder's sidebar group the title of its index page.
			routeMiddleware: './src/sidebarLabels.ts',
			// One group per guide, each generated from its folder: adding a page,
			// or a folder of pages, never edits this file.
			sidebar: [
				{ label: 'User Guide', items: [{ autogenerate: { directory: 'user' } }] },
				{ label: "Developer's Guide", items: [{ autogenerate: { directory: 'developer' } }] },
				{ label: 'Installation Guide', items: [{ autogenerate: { directory: 'install' } }] },
			],
		}),
	],
});
