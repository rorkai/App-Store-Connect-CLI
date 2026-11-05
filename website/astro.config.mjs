import starlight from '@astrojs/starlight';
import { defineConfig } from 'astro/config';
import starlightLlmsTxt from 'starlight-llms-txt';
import starlightSidebarTopics from 'starlight-sidebar-topics';

import navigation from './navigation.json' with { type: 'json' };

export default defineConfig({
	site: 'https://docs.asccli.sh',
	devToolbar: { enabled: false },
	redirects: navigation.redirects,
	integrations: [
		starlight({
			title: 'App Store Connect CLI',
			description: 'Command-line access to the App Store Connect API for iOS, macOS, tvOS, and visionOS release work.',
			favicon: '/favicon.svg',
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/rorkai/App-Store-Connect-CLI' },
				{ icon: 'x.com', label: 'X', href: 'https://x.com/rudrankriyam' },
			],
			customCss: ['@fontsource-variable/inter', './src/styles/custom.css'],
			components: {
				Header: './src/components/overrides/Header.astro',
				PageTitle: './src/components/overrides/PageTitle.astro',
				Sidebar: './src/components/overrides/Sidebar.astro',
			},
			expressiveCode: {
				themes: ['github-dark-default', 'github-light-default'],
				defaultProps: { frame: 'code' },
				styleOverrides: { borderRadius: '0.75rem', codeFontSize: '0.8125rem' },
				shiki: { langAlias: { gitignore: 'txt', strings: 'txt' } },
			},
			plugins: [starlightSidebarTopics(navigation.topics, { exclude: ['index'] }), starlightLlmsTxt()],
		}),
	],
});
