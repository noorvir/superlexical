import { createModels } from "@earendil-works/pi-ai"
import { openaiCodexProvider } from "@earendil-works/pi-ai/providers/openai-codex"
import { togetherProvider } from "@earendil-works/pi-ai/providers/together"

import { getCodexCredentialStore } from "./credentials.ts"
import { env } from "./env.ts"

export const models = createModels({
  credentials: getCodexCredentialStore(env.CODEX_AUTH_FILE),
})
models.setProvider(openaiCodexProvider())
models.setProvider(togetherProvider())

const separator = env.RAY_MODEL.indexOf("/")

/** The model that all Ray agents use, as Pi Durable stores it. */
export const modelRef = {
  provider: env.RAY_MODEL.slice(0, separator),
  modelId: env.RAY_MODEL.slice(separator + 1),
}

export function getModel() {
  const model = models.getModel(modelRef.provider, modelRef.modelId)
  if (!model) {
    throw new Error(`RAY_MODEL names an unknown model: ${env.RAY_MODEL}`)
  }

  return model
}
