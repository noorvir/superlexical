import type { AssistantMessage } from "@earendil-works/pi-ai"
import { watchEvents } from "@earendil-works/pi-durable"
import type {
  AgentEvent,
  EntryRecord,
  SubmissionId,
} from "@earendil-works/pi-durable"

import { modelRef } from "../models.ts"
import type { Search } from "../search/search.ts"
import { context, harness } from "./harness.ts"
import { parseConversationId } from "./ids.ts"
import { readTools } from "./tools.ts"

export type SessionMessage = { role: "user" | "assistant"; text: string }

export type SessionUpdate =
  | { type: "text"; delta: string }
  | { type: "tool"; id: string; name: string; status: "pending" | "completed" }

/**
 * Creates a chat session and returns its ID. When a search is given, the
 * query and its results are the start of the conversation.
 */
export async function createSession(search?: Search) {
  const conversation = await harness.createConversation(
    {
      ownership: { kind: "ownerless" },
      agent: {
        model: modelRef,
        thinkingLevel: "low",
        tools: readTools,
        instructions: instructions(search),
      },
    },
    context
  )
  return String(conversation.id)
}

/** Returns the messages of a session, oldest first, or nothing when the session does not exist. */
export async function getMessages(sessionId: string) {
  const conversation = await getConversation(sessionId)
  if (!conversation) {
    return undefined
  }

  const page = await conversation.entries(
    { order: "ascending" },
    1000,
    undefined,
    context
  )
  return page.items.flatMap(entryMessages)
}

/**
 * Sends one user message to a session and reports the answer as it is made.
 * Resolves when the run ends. The run continues when the caller goes away.
 */
export async function prompt(
  sessionId: string,
  text: string,
  onUpdate: (update: SessionUpdate) => Promise<void>
): Promise<"end_turn" | "cancelled"> {
  const conversation = await getConversation(sessionId)
  if (!conversation) {
    throw new Error(`Session ${sessionId} not found`)
  }

  const stream = await watchEvents(harness, conversation.id, context)
  const settled = Promise.withResolvers<"done" | "unanswered">()
  const sessionUpdates = getSessionUpdates()
  let submissionId: SubmissionId | undefined

  stream.start(async (events) => {
    for (const event of events) {
      const update = sessionUpdates(event)
      for (const item of update) {
        await onUpdate(item)
      }

      if (event.type !== "submission" || event.record.id !== submissionId) {
        continue
      }

      const { status } = event.record
      if (status === "done" || status === "unanswered") {
        settled.resolve(status)
      }
    }
  })

  try {
    const submission = await conversation.submit(
      { type: "input", content: text },
      context
    )
    submissionId = submission.id

    const record = await submission.status(context)
    if (record.status === "done" || record.status === "unanswered") {
      settled.resolve(record.status)
    }

    const status = await settled.promise
    if (status === "done") {
      return "end_turn"
    }

    return "cancelled"
  } finally {
    await stream.stop()
  }
}

export async function cancel(sessionId: string) {
  const conversation = await getConversation(sessionId)
  await conversation?.abort(context)
}

async function getConversation(sessionId: string) {
  const id = parseConversationId(sessionId)
  if (id === undefined) {
    return undefined
  }

  return harness.conversation(id, context)
}

/**
 * Returns a function that converts agent events to session updates. Pi
 * Durable commits partial answers at intervals, so a text block can arrive as
 * a start, as deltas, or only as the finished block. The function tracks how
 * much of each block it reported and reports the remainder.
 */
function getSessionUpdates() {
  let reported = new Map<number, number>()

  const remainder = (index: number, text: string): SessionUpdate[] => {
    const delta = text.slice(reported.get(index) ?? 0)
    reported.set(index, text.length)
    if (!delta) {
      return []
    }

    return [{ type: "text", delta }]
  }

  const blocks = (content: AssistantMessage["content"]) =>
    content.flatMap((block, index) => {
      if (block.type !== "text") {
        return []
      }

      return remainder(index, block.text)
    })

  return (event: AgentEvent): SessionUpdate[] => {
    if (event.type === "message_start") {
      reported = new Map()
      return []
    }

    if (event.type === "message_update") {
      return event.changes.flatMap((change) => {
        if (change.type === "text_delta") {
          const length = reported.get(change.contentIndex) ?? 0
          reported.set(change.contentIndex, length + change.delta.length)
          return [{ type: "text" as const, delta: change.delta }]
        }

        if (change.type === "message") {
          return blocks(change.message.content)
        }

        if (
          (change.type === "text_start" || change.type === "block") &&
          change.block.type === "text"
        ) {
          return remainder(change.contentIndex, change.block.text)
        }

        return []
      })
    }

    if (event.type === "message_end") {
      const message = event.entry.model?.[0]
      if (message?.role !== "assistant") {
        return []
      }

      return blocks(message.content)
    }

    if (event.type === "tool_execution_start") {
      const update = {
        type: "tool" as const,
        id: event.toolCallId,
        name: event.toolName,
        status: "pending" as const,
      }
      return [update]
    }

    if (event.type === "tool_execution_end") {
      const update = {
        type: "tool" as const,
        id: event.toolCallId,
        name: event.toolName,
        status: "completed" as const,
      }
      return [update]
    }

    return []
  }
}

function entryMessages(entry: EntryRecord): SessionMessage[] {
  const messages: SessionMessage[] = []

  for (const message of entry.model ?? []) {
    if (message.role !== "user" && message.role !== "assistant") {
      continue
    }

    let text = ""
    if (typeof message.content === "string") {
      text = message.content
    } else {
      text = message.content
        .flatMap((block) => (block.type === "text" ? [block.text] : []))
        .join("")
    }

    if (text.trim()) {
      messages.push({ role: message.role, text })
    }
  }

  return messages
}

function instructions(search?: Search) {
  const base = `You are Ray, the personal assistant of one person. You can read their knowledge base of linked Markdown files with your tools: lists are in lists/ and general pages are in wiki/. Answer briefly and directly. Use Markdown only when it helps. Refer to a page by its title. Do not write file paths or links to files.`
  if (!search) {
    return base
  }

  const sources = search.text
    .map(
      (result) =>
        `${result.title} (${result.source}, ${result.url}):\n${result.text}`
    )
    .join("\n\n")
  return `${base}

This conversation continues a search. The person searched for: ${search.query}

They saw these results:
${sources || "No results."}

They saw this summary:
${search.summary}

Treat the search and its results as the first part of this conversation.`
}
