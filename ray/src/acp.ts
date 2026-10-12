import * as acp from "@agentclientprotocol/sdk"
import { AcpServer } from "@agentclientprotocol/sdk/experimental/server"

import { cancel, createSession, getMessages, prompt } from "./agent/sessions.ts"

/**
 * The ACP agent of Ray. ACP carries chat sessions only: a session is one
 * durable conversation, and its ID is valid on every connection.
 */
const agent = acp
  .agent({ name: "ray" })
  .onRequest(acp.methods.agent.initialize, () => ({
    protocolVersion: acp.PROTOCOL_VERSION,
    agentCapabilities: { loadSession: true },
  }))
  .onRequest(acp.methods.agent.authenticate, () => ({}))
  .onRequest(acp.methods.agent.session.new, async () => {
    const sessionId = await createSession()
    return { sessionId }
  })
  .onRequest(acp.methods.agent.session.load, async (ctx) => {
    const { sessionId } = ctx.params
    const messages = await getMessages(sessionId)
    if (!messages) {
      throw new Error(`Session ${sessionId} not found`)
    }

    for (const message of messages) {
      let sessionUpdate: "user_message_chunk" | "agent_message_chunk" =
        "agent_message_chunk"
      if (message.role === "user") {
        sessionUpdate = "user_message_chunk"
      }

      await ctx.client.notify(acp.methods.client.session.update, {
        sessionId,
        update: {
          sessionUpdate,
          content: { type: "text", text: message.text },
        },
      })
    }

    return {}
  })
  .onRequest(acp.methods.agent.session.prompt, async (ctx) => {
    const { sessionId } = ctx.params
    const text = ctx.params.prompt
      .flatMap((block) => (block.type === "text" ? [block.text] : []))
      .join("\n")

    const stopReason = await prompt(sessionId, text, async (update) => {
      if (update.type === "text") {
        await ctx.client.notify(acp.methods.client.session.update, {
          sessionId,
          update: {
            sessionUpdate: "agent_message_chunk",
            content: { type: "text", text: update.delta },
          },
        })
        return
      }

      if (update.status === "pending") {
        await ctx.client.notify(acp.methods.client.session.update, {
          sessionId,
          update: {
            sessionUpdate: "tool_call",
            toolCallId: update.id,
            title: update.name,
            kind: "read",
            status: "pending",
          },
        })
        return
      }

      await ctx.client.notify(acp.methods.client.session.update, {
        sessionId,
        update: {
          sessionUpdate: "tool_call_update",
          toolCallId: update.id,
          status: "completed",
        },
      })
    })

    return { stopReason }
  })
  .onNotification(acp.methods.agent.session.cancel, async (ctx) => {
    await cancel(ctx.params.sessionId)
  })

export const acpServer = new AcpServer({ agent })
