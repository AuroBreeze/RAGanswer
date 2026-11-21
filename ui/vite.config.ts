import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-auto';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), 'RAG_');
	const demoUser = env.RAG_DEV_USER_ID || process.env.RAG_DEV_USER_ID || 'local-demo';
	const apiTarget = env.RAG_API_URL || process.env.RAG_API_URL || 'http://127.0.0.1:8080';

	return {
		server: {
			host: '127.0.0.1',
			proxy: {
				'/api/v1/chat': {
					target: apiTarget
				}
			}
		},
		plugins: [
			{
				name: 'rag-dev-identity',
				configureServer(server) {
					server.middlewares.use((request, _response, next) => {
						if (request.url?.startsWith('/api/v1/chat')) {
							request.headers['x-user-id'] = demoUser;
						}
						next();
					});
				}
			},
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
