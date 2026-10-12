import { z } from "zod"

import { env } from "./env.ts"

const listSchema = z.object({
  slug: z.string(),
  name: z.string(),
  description: z.string(),
  entryCount: z.number(),
})
const entrySchema = z.object({ path: z.string(), title: z.string() })
const changeResultSchema = z.object({
  applied: z.boolean(),
  commit: z.string(),
  rejections: z.array(z.object({ path: z.string(), message: z.string() })),
  diagnostics: z.array(
    z.object({
      path: z.string(),
      line: z.number(),
      field: z.string(),
      message: z.string(),
    })
  ),
})

/** A set of edits that Core applies as one commit. */
export type Change = {
  author: string
  message: string
  /** Time when the author sent the change. Core rejects it when a file changed after that time. */
  sentAt: Date
  creates: { path: string; content: string }[]
  replacements: { path: string; oldText: string; newText: string }[]
}

export type List = z.infer<typeof listSchema>

export async function getLists() {
  const body = await request("/lists")
  return z.object({ lists: z.array(listSchema) }).parse(body).lists
}

export async function getList(slug: string) {
  const body = await request(`/lists/${encodeURIComponent(slug)}`)
  const schema = z.object({ list: listSchema, entries: z.array(entrySchema) })
  return schema.parse(body)
}

export async function hasPage(path: string) {
  const url = new URL("/page", env.SUPERLEXICAL_API_URL)
  url.searchParams.set("path", path)
  const response = await fetch(url)
  return response.ok
}

/**
 * Asks Core to apply a change. Core is the only writer of the workspace. A
 * rejected change modifies nothing; the result says why Core rejected it.
 */
export async function applyChange(change: Change) {
  const body = await request("/changes", change)
  return changeResultSchema.parse(body)
}

async function request(path: string, body?: object): Promise<unknown> {
  const url = new URL(path, env.SUPERLEXICAL_API_URL)
  let init: RequestInit = {}
  if (body) {
    init = {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    }
  }

  const response = await fetch(url, init)
  if (!response.ok) {
    throw new Error(`Core ${path} failed with status ${response.status}`)
  }

  return response.json()
}
