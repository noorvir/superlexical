import { chmod, readFile, rename, writeFile } from "node:fs/promises"
import type { Credential, CredentialStore } from "@earendil-works/pi-ai"
import { z } from "zod"

const providerId = "openai-codex"

const authFileSchema = z.looseObject({
  tokens: z.looseObject({
    access_token: z.string(),
    refresh_token: z.string(),
    account_id: z.string(),
  }),
})

/**
 * Returns a credential store backed by the `auth.json` file of the Codex CLI.
 * It holds one credential, for the `openai-codex` provider.
 *
 * A refreshed token is written back to the file, because the provider rotates
 * the refresh token and the old one stops working for the Codex CLI too.
 */
export function getCodexCredentialStore(path: string): CredentialStore {
  let writes = Promise.resolve()

  const read = async (): Promise<Credential> => {
    const file = await readAuthFile(path)
    const { access_token, refresh_token, account_id } = file.tokens
    return {
      type: "oauth",
      access: access_token,
      refresh: refresh_token,
      expires: expiryTime(access_token),
      accountId: account_id,
    }
  }

  return {
    async read(id) {
      if (id !== providerId) {
        return undefined
      }

      return read()
    },

    async list() {
      return [{ providerId, type: "oauth" }]
    },

    async modify(id, fn) {
      if (id !== providerId) {
        return undefined
      }

      const write = writes.then(async () => {
        const current = await read()
        const next = await fn(current)
        if (!next || next.type !== "oauth") {
          return current
        }

        const file = await readAuthFile(path)
        file.tokens.access_token = next.access
        file.tokens.refresh_token = next.refresh
        file.last_refresh = new Date().toISOString()
        await writeAuthFile(path, file)
        return next
      })
      writes = write.then(
        () => undefined,
        () => undefined
      )
      return write
    },

    async delete() {
      throw new Error(
        "Ray does not remove credentials from the Codex auth file"
      )
    },
  }
}

async function readAuthFile(path: string) {
  const source = await readFile(path, "utf8")
  return authFileSchema.parse(JSON.parse(source))
}

async function writeAuthFile(
  path: string,
  file: z.infer<typeof authFileSchema>
) {
  const temporary = `${path}.ray-${process.pid}`
  await writeFile(temporary, JSON.stringify(file, null, 2), { mode: 0o600 })
  await chmod(temporary, 0o600)
  await rename(temporary, path)
}

/** Reads the expiry time, in milliseconds, from the payload of a JWT. Returns 0 for a token that it cannot read. */
function expiryTime(token: string) {
  const payload = token.split(".")[1] ?? ""
  const claims = z
    .object({ exp: z.number() })
    .safeParse(parseJson(Buffer.from(payload, "base64url").toString("utf8")))
  if (!claims.success) {
    return 0
  }

  return claims.data.exp * 1000
}

function parseJson(source: string): unknown {
  try {
    return JSON.parse(source)
  } catch {
    return undefined
  }
}
