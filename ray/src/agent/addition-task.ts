import { defineExtension, defineTask } from "@earendil-works/pi-durable"

import { hasPage } from "../core.ts"
import { entryPaths } from "./tools.ts"

export type AdditionInput = {
  list: string
  title: string
  /** Workspace-relative path of the entry that the agent writes. */
  path: string
}

/**
 * The durable task of one list addition. It owns the work of the agent in
 * its conversation: it sends the instruction, waits for the run, and records
 * whether Core has the entry. Pi Durable continues the task after a restart.
 */
export const AdditionTask = defineTask<
  AdditionInput,
  { phase: "run" },
  { path: string }
>({
  name: "ray.addition",
  version: 1,
  initial: () => ({ phase: "run" }),
  phases: {
    run: async (task, runtime, context) => {
      const { title, path } = task.input
      const conversationId = String(runtime.conversationId)
      entryPaths.set(conversationId, path)

      try {
        const conversation = await runtime.conversation(
          runtime.conversationId,
          context
        )
        if (!conversation) {
          throw new Error("The conversation of this addition does not exist")
        }

        // The request ID makes a rerun after a restart find the same submission.
        const submission = await conversation.submit(
          {
            type: "input",
            content: `Add "${title}" to the list now.`,
            requestId: `addition:${runtime.taskId}`,
          },
          context
        )
        const settled = await submission.wait(context)
        const committed = await hasPage(path)
        if (settled.status !== "done" || !committed) {
          throw new Error("The agent did not commit the entry")
        }

        await runtime.commit(
          () => ({
            status: "terminal",
            outcome: { status: "completed", result: { path } },
          }),
          context
        )
      } catch (error) {
        if (runtime.signal.aborted) {
          throw error
        }

        console.error(`Ray cannot add ${title}:`, error)
        await runtime.commit(
          () => ({
            status: "terminal",
            outcome: { status: "failed", error: { message: String(error) } },
          }),
          context
        )
      } finally {
        entryPaths.delete(conversationId)
      }
    },
  },
  abort: async (_task, runtime, context) => {
    await runtime.commit(
      () => ({ status: "terminal", outcome: { status: "aborted" } }),
      context
    )
  },
})

export const Additions = defineExtension({
  name: "additions",
  tasks: [AdditionTask],
})
