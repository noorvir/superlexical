import { defineConfig } from "vite"
import { tanstackStart } from "@tanstack/react-start/plugin/vite"
import viteReact from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"

const port = Number(process.env.PRISM_WEB_PORT ?? "4300")
const coreUrl = process.env.SUPERLEXICAL_API_URL ?? "http://127.0.0.1:4100"
const rayUrl = process.env.RAY_URL ?? "http://127.0.0.1:4200"

export default defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [tailwindcss(), tanstackStart(), viteReact()],
  server: {
    host: "127.0.0.1",
    port,
    strictPort: true,
    proxy: {
      "/api": {
        target: coreUrl,
        rewrite: (path) => path.replace(/^\/api/, ""),
      },
      "/ray": {
        target: rayUrl,
        rewrite: (path) => path.replace(/^\/ray/, ""),
      },
    },
  },
})
