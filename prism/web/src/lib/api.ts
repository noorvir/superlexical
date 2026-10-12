import createClient from "openapi-fetch"

import type { paths } from "./schema"

const core = createClient<paths>({ baseUrl: "/api" })

export async function getCoreHealth() {
  const { data, error } = await core.GET("/health")
  if (error) {
    throw new Error("Core is not reachable")
  }

  return data
}

export async function getLists() {
  const { data, error } = await core.GET("/lists")
  if (error) {
    throw new Error("Core cannot read the lists")
  }

  return data.lists
}

export async function getRayHealth() {
  const response = await fetch("/ray/health")
  if (!response.ok) {
    throw new Error("Ray is not reachable")
  }
}
