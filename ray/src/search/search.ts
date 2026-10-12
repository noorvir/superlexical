import { randomUUID } from "node:crypto"

import { getList, getLists } from "../core.ts"
import { getModel, models } from "../models.ts"
import type { ImageResult, TextResult } from "./provider.ts"
import { wikipedia } from "./wikipedia.ts"

const textProviders = [wikipedia]
const imageProviders = [wikipedia]

export type SearchEvent =
  | { type: "result"; text: TextResult[]; images: ImageResult[] }
  | { type: "summary"; delta: string }
  | { type: "suggestion"; list: { slug: string; name: string }; title: string }
  | { type: "done"; searchId: string }
  | { type: "error"; message: string }

/** A finished search, kept so that it can become a chat session. */
export type Search = {
  id: string
  query: string
  text: TextResult[]
  summary: string
}

const searches = new Map<string, Search>()

export function getSearch(id: string) {
  return searches.get(id)
}

/**
 * Runs one search and reports its parts as they become available: provider
 * results first, then the model summary, then the list suggestion.
 */
export async function runSearch(
  query: string,
  emit: (event: SearchEvent) => Promise<void>,
  signal: AbortSignal
) {
  const textSearches = textProviders.map((provider) =>
    provider.searchText(query, signal)
  )
  const imageSearches = imageProviders.map((provider) =>
    provider.searchImages(query, signal)
  )
  const textResults = await Promise.all(textSearches)
  const imageResults = await Promise.all(imageSearches)
  const text = textResults.flat()
  const images = imageResults.flat()
  await emit({ type: "result", text, images })

  const title = text[0]?.title ?? query
  const suggestion = suggestList(title, text, signal).catch((error) => {
    console.error("Ray cannot suggest a list:", error)
    return undefined
  })

  const summary = await summarize(query, text, emit, signal)
  const suggested = await suggestion
  if (suggested) {
    await emit({ type: "suggestion", list: suggested, title })
  }

  const search = { id: randomUUID(), query, text, summary }
  searches.set(search.id, search)
  await emit({ type: "done", searchId: search.id })
}

async function summarize(
  query: string,
  text: TextResult[],
  emit: (event: SearchEvent) => Promise<void>,
  signal: AbortSignal
) {
  const sources = text
    .map((result) => `${result.title} (${result.source}):\n${result.text}`)
    .join("\n\n")
  const stream = models.streamSimple(
    getModel(),
    {
      systemPrompt:
        "You answer a search query in two or three short sentences. Give the facts only. Do not explain your method, do not greet, and do not use headings or lists. Use the search results when they are given.",
      messages: [
        {
          role: "user",
          content: `Query: ${query}\n\nSearch results:\n${sources || "None."}`,
          timestamp: Date.now(),
        },
      ],
    },
    { signal, reasoning: "minimal" }
  )

  let summary = ""
  for await (const event of stream) {
    if (event.type === "text_delta") {
      summary += event.delta
      await emit({ type: "summary", delta: event.delta })
    }
    if (event.type === "error") {
      throw new Error(event.error.errorMessage ?? "The model call failed")
    }
  }

  return summary
}

/**
 * Returns the list that the topic belongs to, or nothing when no list fits or
 * when the list already has an entry with that title.
 */
async function suggestList(
  title: string,
  text: TextResult[],
  signal: AbortSignal
) {
  const lists = await getLists()
  if (lists.length === 0) {
    return undefined
  }

  const choices = lists
    .map((list) => `- ${list.slug}: ${list.name}. ${list.description}`)
    .join("\n")
  const answer = await models.completeSimple(
    getModel(),
    {
      systemPrompt:
        "You file a topic into one of the lists of a person. Reply with the slug of the single list that the topic clearly belongs to. Reply with the word none when no list fits. Reply with one word only.",
      messages: [
        {
          role: "user",
          content: `Topic: ${title}\n${text[0]?.text.slice(0, 500) ?? ""}\n\nLists:\n${choices}`,
          timestamp: Date.now(),
        },
      ],
    },
    { signal, reasoning: "minimal" }
  )

  const reply = answer.content
    .flatMap((block) => (block.type === "text" ? [block.text] : []))
    .join("")
    .trim()
    .toLowerCase()
  const list = lists.find((candidate) => candidate.slug === reply)
  if (!list) {
    return undefined
  }

  const { entries } = await getList(list.slug)
  const exists = entries.some(
    (entry) => entry.title.toLowerCase() === title.toLowerCase()
  )
  if (exists) {
    return undefined
  }

  return { slug: list.slug, name: list.name }
}
