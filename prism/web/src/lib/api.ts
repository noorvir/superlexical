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

export async function getDiagnostics() {
  const { data, error } = await core.GET("/workspace/diagnostics")
  if (error) {
    throw new Error("Core cannot validate the workspace")
  }

  return data.diagnostics
}

export async function getRayHealth() {
  const response = await fetch("/ray/health")
  if (!response.ok) {
    throw new Error("Ray is not reachable")
  }
}
