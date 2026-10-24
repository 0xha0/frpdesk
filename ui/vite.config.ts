import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";

// dev: `npm run dev` serves the app and passes /api to a running frpdesk (its UI port)
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  build: { outDir: "dist", emptyOutDir: true, sourcemap: false },
  server: { proxy: { "/api": { target: "http://127.0.0.1:7201", changeOrigin: false } } },
});
