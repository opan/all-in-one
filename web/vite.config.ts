import adapter from "@sveltejs/adapter-static";
import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";
import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig } from "vite";

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			preprocess: vitePreprocess(),
			adapter: adapter({
				fallback: "index.html" /* SPA mode: Go backend serves index.html for unknown routes */
			})
		})
	],
	server: {
		proxy: {
			// Proxy API requests (WebSocket connects directly to backend in dev)
			"/api": {
				target: "http://localhost:8080",
				changeOrigin: true,
			},
			"/r": {
				target: "http://localhost:8080",
				changeOrigin: true,
			},
		},
		watch: {
			usePolling: true,
			interval: 100,
		},
		hmr: {
			overlay: true,
		},
	},
});
