import { mkdir } from "node:fs/promises"
import { join } from "node:path"

import { BACKGROUND_CONTEXT } from "@earendil-works/chord/context"
import { createRegistry, Harness } from "@earendil-works/pi-durable"
import { openNodeSqliteStorage } from "@earendil-works/pi-durable/storage/sqlite/node"

import { env } from "../env.ts"
import { models } from "../models.ts"
import { Additions } from "./addition-task.ts"
import { Workspace } from "./tools.ts"

/** Ray does not cancel durable work when a client goes away. */
export const context = BACKGROUND_CONTEXT

await mkdir(env.RAY_DATA_DIR, { recursive: true })

const registry = createRegistry()
registry.install(Workspace)
registry.install(Additions)

const storage = await openNodeSqliteStorage(
  join(env.RAY_DATA_DIR, "ray.sqlite")
)

export const harness = await Harness.open(
  storage,
  {
    models,
    registry,
    settings: { toolExecution: "sequential" },
    onReport: (error) => console.error("Pi Durable:", error),
  },
  context
)
