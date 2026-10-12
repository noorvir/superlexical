import { z } from "zod"

import { getList } from "../core.ts"
import { modelRef } from "../models.ts"
import { AdditionTask } from "./addition-task.ts"
import { context, harness } from "./harness.ts"
import { writeTools } from "./tools.ts"

export type Addition = {
  id: string
  list: string
  title: string
  /** Workspace-relative path of the entry that the agent writes. */
  path: string
  status: "running" | "done" | "failed"
}

const inputSchema = z.object({
  list: z.string(),
  title: z.string(),
  path: z.string(),
})

/** Returns all additions, oldest first, from the task records of Pi Durable. */
export async function getAdditions() {
  const query = { kind: AdditionTask.definition.name }
  const page = await harness.commit((tx) => tx.scanTasks(query, 500), context)

  return page.items.map((task): Addition => {
    const input = inputSchema.parse(task.input)
    let status: Addition["status"] = "running"
    if (task.state.status === "terminal") {
      status = "failed"
    }
    if (task.state.outcome?.status === "completed") {
      status = "done"
    }

    return { id: String(task.id), ...input, status }
  })
}

/**
 * Starts a background agent that adds one entry to a list. Returns as soon as
 * Pi Durable has recorded the task. Additions run at the same time: each agent
 * sends its edits to Core, and Core applies one change at a time.
 */
export async function startAddition(list: string, title: string) {
  const { list: found, entries } = await getList(list)
  const slug = title
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "")
  const path = `lists/${found.slug}/${slug}.md`
  const exists = entries.some((entry) => entry.path === path)
  if (!slug || exists) {
    return undefined
  }

  // The conversation of the agent and its task are created in one commit.
  let taskId = ""
  await harness.createConversation(
    {
      ownership: { kind: "ownerless" },
      agent: {
        model: modelRef,
        thinkingLevel: "low",
        tools: writeTools,
        instructions: instructions(found.name, path, title),
      },
      init: async (tx, conversationId) => {
        const input = { list: found.slug, title, path }
        const options = {
          conversationId,
          ownership: { kind: "conversation" as const },
          background: true,
        }
        const id = await tx.createTask(AdditionTask, input, options)
        taskId = String(id)
      },
    },
    context
  )

  // The scheduler starts when it is asked to; a new task alone does not start it.
  harness.resume()
  return { id: taskId, status: "running" as const }
}

function instructions(listName: string, path: string, title: string) {
  const index = path.replace(/[^/]+$/, "index.md")
  return `You maintain a personal knowledge base of linked Markdown files. Your task is to add one new entry, "${title}", to the list "${listName}", and to connect it to the knowledge that already exists. Work in the same way as a careful Wikipedia editor.

The workspace has lists in lists/<list>/ and general pages in wiki/. Each file starts with YAML front matter. Links are relative Markdown links, such as [Rome](../../wiki/rome.md).

Do these steps in order.

1. Read ${index}. Its schema field gives the front matter fields of an entry and which fields are required. Read one existing entry of the list to see the style.
2. Decide the text of the entry before you look for links. Write front matter that satisfies the schema, a level-one heading with the title, and one or two short paragraphs of plain facts: who or what this is, and what it is best known for. Use your own knowledge. Do not add fields that the schema does not define.
3. Find related knowledge. Call list_files to see every page. Use search_files for the important names, places, and concepts in your text. Read the pages that look relevant.
4. Prepare the entry. Link a term to an existing page only when that page is clearly about the term and the link helps a reader. Link each page one time only, at the first mention. Most terms have no page; leave them as plain text.
5. Prepare the reverse links. For each existing page that mentions "${title}" or is closely related to it, prepare one replacement that links to the new entry at the place where it fits. If a closely related page has no suitable sentence, add one short sentence or list item to it. Change only what is necessary. Do not add a reverse link where it is not relevant.
6. Call apply_changes one time, with the entry and all replacements together. They become one commit. If the reply says that the change was rejected, read the named files again, correct the edits, and call apply_changes again with the entry and all replacements.

Rules that you must not break:

- Never create a page. The only new file is ${path}, and you create it with the entry parameter of apply_changes. Link only to files that list_files returned.
- Write relative link paths from the folder of the file that contains the link.
- Do not link to the same page more than one time in a file.
- Stop after apply_changes succeeds. Then reply with one sentence that says what you linked.`
}
