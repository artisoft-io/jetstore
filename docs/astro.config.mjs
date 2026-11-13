// @ts-check
// JetStore's documentation site. Served by the apiserver at /docs/ on an
// installation, from the directory the UI image copies dist/ to.
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightLinksValidator from 'starlight-links-validator';
import { existsSync, readFileSync } from 'node:fs';

// A workspace's guide, written into src/content/docs/workspace/ by `npm run gen`
// when JETS_WORKSPACE_DB is set (scripts/workspace_docs.mjs), is a fourth group,
// labelled with the workspace's name. Without one the site is the three guides.
const workspaceGuide = existsSync(new URL('./src/content/docs/workspace/', import.meta.url));
function workspaceName() {
	try {
		const manifest = JSON.parse(readFileSync(new URL('./public/workspace/manifest.json', import.meta.url), 'utf8'));
		if (typeof manifest.workspace === 'string' && manifest.workspace) return manifest.workspace;
	} catch {
		// No manifest: fall back to the environment.
	}
	return process.env.JETS_WORKSPACE_NAME?.trim() || 'Workspace';
}

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
			// public/favicon.svg, served as /docs/favicon.svg: an i in the JetStore cube.
			favicon: '/favicon.svg',
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
				...(workspaceGuide ? [{ label: workspaceName(), items: [{ autogenerate: { directory: 'workspace' } }] }] : []),
			],
			// The footer names the JetStore and workspace versions the site was built for.
			components: { Footer: './src/components/Footer.astro' },
		}),
	],
});
