import { homedir } from "node:os"
import { join, resolve } from "node:path"

const workspace = process.env.RAY_WORKSPACE
if (!workspace) {
  throw new Error(
    "RAY_WORKSPACE must be the path of the Superlexical workspace"
  )
}

export const env = {
  RAY_HOST: process.env.RAY_HOST ?? "127.0.0.1",
  RAY_PORT: Number(process.env.RAY_PORT ?? "4200"),
  RAY_WORKSPACE: resolve(workspace),
  RAY_DATA_DIR: resolve(process.env.RAY_DATA_DIR ?? ".ray"),
  /** `provider/modelId` of a model that a registered provider serves. */
  RAY_MODEL: process.env.RAY_MODEL ?? "openai-codex/gpt-5.6-luna",
  SUPERLEXICAL_API_URL:
    process.env.SUPERLEXICAL_API_URL ?? "http://127.0.0.1:4100",
  /** Login of the Codex CLI. Ray uses it for `openai-codex` models. */
  CODEX_AUTH_FILE:
    process.env.CODEX_AUTH_FILE ?? join(homedir(), ".codex", "auth.json"),
}
