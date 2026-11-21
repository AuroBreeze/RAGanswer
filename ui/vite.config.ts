import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-auto';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), 'RAG_');
	const demoUser = env.RAG_DEV_USER_ID;
	return {
		server: {
			host: '127.0.0.1',
			proxy: {
				'/internal/v1/admin/': { target: env.RAG_API_URL || 'http://127.0.0.1:8080' },
				'/api/v1/chat': {
					target: env.RAG_API_URL || 'http://127.0.0.1:8080',
					configure(proxy) {
						proxy.on('proxyReq', (request) => {
							for (const header of [
								'x-user-id',
								'x-user-roles',
								'x-user-factory',
								'x-user-workshop'
							]) {
								request.removeHeader(header);
							}
							if (demoUser) request.setHeader('x-user-id', demoUser);
						});
					}
				}
			}
		},
		plugins: [
			tailwindcss(),
			sveltekit({
				compilerOptions: {
					// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
					runes: ({ filename }) =>
						filename.split(/[/\\]/).includes('node_modules') ? undefined : true
				},

				// adapter-auto only supports some environments, see https://svelte.dev/docs/kit/adapter-auto for a list.
				// If your environment is not supported, or you settled on a specific environment, switch out the adapter.
				// See https://svelte.dev/docs/kit/adapters for more information about adapters.
				adapter: adapter()
			})
		]
	};
});
