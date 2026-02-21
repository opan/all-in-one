import tailwindcss from "@tailwindcss/vite";
import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig } from "vite";

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			"/api": {
				target: "http://localhost:8080",
				changeOrigin: true,
				ws: true, // Enable WebSocket proxying
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
