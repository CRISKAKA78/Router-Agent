import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
export default defineConfig({
  base: "./",
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      "/api": {
        target: process.env.RMP_SERVER ?? "http://127.0.0.1:8080",
        changeOrigin: true,
        ws: true,
        rewrite: (p) => p,
        configure: (proxy) => {
          proxy.on("proxyReq", (req) => req.removeHeader("origin"));
          proxy.on("proxyReqWs", (req) => req.removeHeader("origin"));
        },
      },
    },
  },
});
