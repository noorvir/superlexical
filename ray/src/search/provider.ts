/** A text result: a page that answers the query. */
export type TextResult = {
  title: string
  /** Plain text extract of the page, without markup. */
  text: string
  url: string
  /** Name of the site or service that has the page. */
  source: string
}

export type ImageResult = {
  url: string
  thumbnailUrl: string
  /** Page that shows the image. */
  pageUrl: string
  title: string
}

/**
 * A source of search results. Text and image search are separate so that a
 * provider can supply one or the two, and so that Ray can combine providers.
 */
export interface TextSearchProvider {
  searchText(query: string, signal: AbortSignal): Promise<TextResult[]>
}

export interface ImageSearchProvider {
  searchImages(query: string, signal: AbortSignal): Promise<ImageResult[]>
}
