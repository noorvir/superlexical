import { env } from "./env.ts"

const server = Bun.serve({
  hostname: env.RAY_HOST,
  port: env.RAY_PORT,
  routes: {
    "/health": {
      GET: () => Response.json({ status: "ok" }),
    },
  },
  fetch() {
    return Response.json({ message: "Not found" }, { status: 404 })
  },
})

console.log(`Ray listening on ${server.url}`)
