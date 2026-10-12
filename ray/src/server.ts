import { createNodeWebSocketUpgradeHandler } from "@agentclientprotocol/sdk/experimental/node"
import { serve } from "@hono/node-server"
import { Hono } from "hono"
import { streamSSE } from "hono/streaming"
import { WebSocketServer } from "ws"

import { acpServer } from "./acp.ts"
import { getAdditions, startAddition } from "./agent/additions.ts"
import { createSession } from "./agent/sessions.ts"
import { env } from "./env.ts"
import { getSearch, runSearch } from "./search/search.ts"

const app = new Hono()

app.get("/health", (c) => c.json({ status: "ok" }))

app.post("/search", async (c) => {
  const body = await c.req.json().catch(() => undefined)
  const query = typeof body?.query === "string" ? body.query.trim() : ""
  if (!query) {
    return c.json({ message: "query must be a non-empty string" }, 400)
  }

  return streamSSE(c, async (stream) => {
    const aborted = new AbortController()
    stream.onAbort(() => aborted.abort())

    try {
      await runSearch(
        query,
        async (event) => {
          await stream.writeSSE({
            event: event.type,
            data: JSON.stringify(event),
          })
        },
        aborted.signal
      )
    } catch (error) {
      if (aborted.signal.aborted) {
        return
      }

      console.error("Ray cannot complete the search:", error)
      const event = { type: "error", message: "The search did not complete." }
      await stream.writeSSE({ event: "error", data: JSON.stringify(event) })
    }
  })
})

app.post("/searches/:id/session", async (c) => {
  const search = getSearch(c.req.param("id"))
  if (!search) {
    return c.json({ message: "search not found" }, 404)
  }

  const sessionId = await createSession(search)
  return c.json({ sessionId })
})

app.get("/additions", async (c) => {
  const additions = await getAdditions()
  return c.json({ additions })
})

app.post("/additions", async (c) => {
  const body = await c.req.json().catch(() => undefined)
  const list = typeof body?.list === "string" ? body.list : ""
  const title = typeof body?.title === "string" ? body.title.trim() : ""
  if (!list || !title) {
    return c.json({ message: "list and title must be non-empty strings" }, 400)
  }

  const addition = await startAddition(list, title)
  if (!addition) {
    return c.json({ message: "the list already has this entry" }, 409)
  }

  return c.json({ id: addition.id, status: addition.status }, 202)
})

export function startServer() {
  const server = serve({
    fetch: app.fetch,
    hostname: env.RAY_HOST,
    port: env.RAY_PORT,
  })

  const sockets = new WebSocketServer({
    noServer: true,
    maxPayload: 16 * 1024 * 1024,
  })
  const upgrade = createNodeWebSocketUpgradeHandler(acpServer, sockets)

  server.on("upgrade", (request, socket, head) => {
    const { pathname } = new URL(request.url ?? "/", "http://ray")
    if (pathname !== "/acp") {
      socket.destroy()
      return
    }

    upgrade(request, socket, head)
  })

  return server
}
