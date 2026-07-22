import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Dev-only convenience: proxy /api and /v1/opamp to a local server run
// with `go run ./cmd/opamp-server`, so `npm run dev` works without CORS
// configuration. Production builds are static files served by the
// Dockerfile's nginx config, which does its own reverse-proxying to the
// server Service (see ui/nginx.conf).
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
  build: {
    outDir: "dist",
    sourcemap: false,
  },
});
