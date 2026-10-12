import { z } from "zod"

import type {
  ImageResult,
  ImageSearchProvider,
  TextResult,
  TextSearchProvider,
} from "./provider.ts"

const headers = {
  "user-agent": "Ray/0.1 (Superlexical MVP)",
  accept: "application/json",
}

const querySchema = z.object({
  query: z
    .object({
      pages: z.record(
        z.string(),
        z.object({
          title: z.string(),
          extract: z.string().optional(),
          fullurl: z.string(),
        })
      ),
    })
    .optional(),
})

const mediaListSchema = z.object({
  items: z.array(
    z.object({
      type: z.string(),
      title: z.string(),
      showInGallery: z.boolean().optional(),
      srcset: z
        .array(z.object({ src: z.string(), scale: z.string() }))
        .optional(),
    })
  ),
})

export const wikipedia: TextSearchProvider & ImageSearchProvider = {
  async searchText(query, signal) {
    const page = await findPage(query, signal)
    if (!page) {
      return []
    }

    const result: TextResult = {
      title: page.title,
      text: page.extract ?? "",
      url: page.fullurl,
      source: "Wikipedia",
    }
    return [result]
  },

  async searchImages(query, signal) {
    const page = await findPage(query, signal)
    if (!page) {
      return []
    }

    const title = encodeURIComponent(page.title.replaceAll(" ", "_"))
    const url = `https://en.wikipedia.org/api/rest_v1/page/media-list/${title}`
    const response = await fetch(url, { headers, signal })
    if (!response.ok) {
      return []
    }

    const body = await response.json()
    const media = mediaListSchema.parse(body)
    const images: ImageResult[] = []

    for (const item of media.items) {
      const sources = item.srcset ?? []
      const first = sources[0]
      const last = sources.at(-1)
      if (item.type !== "image" || !item.showInGallery || !first || !last) {
        continue
      }

      images.push({
        url: `https:${last.src}`,
        thumbnailUrl: `https:${first.src}`,
        pageUrl: `https://en.wikipedia.org/wiki/${encodeURIComponent(item.title)}`,
        title: item.title.replace(/^File:/, "").replace(/\.\w+$/, ""),
      })
    }

    return images.slice(0, 8)
  },
}

const pages = new Map<string, ReturnType<typeof fetchPage>>()

/** The text and image searches of one query share one page lookup. */
function findPage(query: string, signal: AbortSignal) {
  let page = pages.get(query)
  if (!page) {
    page = fetchPage(query, signal)
    pages.set(query, page)
    void page.finally(() =>
      setTimeout(() => pages.delete(query), 60_000).unref()
    )
  }

  return page
}

async function fetchPage(query: string, signal: AbortSignal) {
  const url = new URL("https://en.wikipedia.org/w/api.php")
  url.search = new URLSearchParams({
    action: "query",
    format: "json",
    generator: "search",
    gsrsearch: query,
    gsrlimit: "1",
    prop: "extracts|info",
    exintro: "1",
    explaintext: "1",
    inprop: "url",
    redirects: "1",
  }).toString()

  const response = await fetch(url, { headers, signal })
  if (!response.ok) {
    throw new Error(`Wikipedia search failed with status ${response.status}`)
  }

  const body = await response.json()
  const found = Object.values(querySchema.parse(body).query?.pages ?? {})
  return found[0]
}
