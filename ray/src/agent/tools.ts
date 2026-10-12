import { execFile } from "node:child_process"
import { readdir, readFile } from "node:fs/promises"
import { join, relative, resolve, sep } from "node:path"
import { promisify } from "node:util"

import { Type } from "@earendil-works/pi-ai"
import { defineExtension, defineTool } from "@earendil-works/pi-durable"

import { applyChange } from "../core.ts"
import { env } from "../env.ts"

const run = promisify(execFile)

/**
 * The path of the one file that a conversation may create. Ray sets it when
 * it starts an addition. A conversation without an entry cannot create files.
 *
 * The tools read the workspace files directly. They never write them: all
 * changes go to Core, which is the only writer.
 */
export const entryPaths = new Map<string, string>()

const listFiles = defineTool({
  name: "list_files",
  description:
    "List the Markdown files of the workspace, or of one folder of it.",
  parameters: Type.Object({
    folder: Type.Optional(
      Type.String({ description: "Workspace-relative folder, such as wiki" })
    ),
  }),
  replay: "safe",
  execute: async (args) => {
    const root = workspacePath(args.folder ?? ".")
    const names = await readdir(root, { recursive: true })
    const files = names
      .filter((name) => name.endsWith(".md") && !name.startsWith(".git"))
      .map((name) => relative(env.RAY_WORKSPACE, join(root, name)))
      .sort()
    return text(files.join("\n") || "No Markdown files.")
  },
})

const readFileTool = defineTool({
  name: "read_file",
  description: "Read one file of the workspace.",
  parameters: Type.Object({
    path: Type.String({ description: "Workspace-relative path" }),
  }),
  replay: "safe",
  execute: async (args) => {
    const source = await readFile(workspacePath(args.path), "utf8")
    return text(source)
  },
})

const searchFiles = defineTool({
  name: "search_files",
  description:
    "Search the Markdown files of the workspace for a word or phrase, without regard to case. Returns the matching lines with their file and line number.",
  parameters: Type.Object({
    text: Type.String({ description: "Word or phrase to find" }),
  }),
  replay: "safe",
  execute: async (args) => {
    const command = ["grep", "-n", "-i", "-I", "-F", "-e", args.text, "--", "."]
    const found = await run("git", command, { cwd: env.RAY_WORKSPACE }).catch(
      () => ({ stdout: "" })
    )
    return text(found.stdout.slice(0, 8000) || "No matches.")
  },
})

const applyChanges = defineTool({
  name: "apply_changes",
  description:
    "Change the workspace. All edits of one call become one commit, or none of them is applied. The change is rejected when an edit does not fit the current files, when another author changed one of the files after you sent the call, or when the result fails validation. The reply gives the reasons. Read the files again, correct the edits, and call again.",
  parameters: Type.Object({
    message: Type.String({ description: "One line that says what changed" }),
    entry: Type.Optional(
      Type.String({
        description:
          "Complete content of the new list entry: front matter and Markdown body. This is the only file that you can create.",
      })
    ),
    replacements: Type.Optional(
      Type.Array(
        Type.Object({
          path: Type.String({
            description: "Workspace-relative path of an existing file",
          }),
          old_text: Type.String({
            description: "Text that occurs exactly once in the file",
          }),
          new_text: Type.String(),
        })
      )
    ),
  }),
  execute: async (args, api) => {
    const sentAt = new Date()
    const conversationId = String(api.conversationId)
    const creates = []

    if (args.entry !== undefined) {
      const path = entryPaths.get(conversationId)
      if (!path) {
        return failure("This conversation cannot create files.")
      }

      creates.push({ path, content: args.entry })
    }

    const replacements = (args.replacements ?? []).map((replacement) => ({
      path: replacement.path,
      oldText: replacement.old_text,
      newText: replacement.new_text,
    }))

    const result = await applyChange({
      author: `Ray (conversation ${conversationId})`,
      message: args.message,
      sentAt,
      creates,
      replacements,
    })
    if (result.applied) {
      return text("Applied and committed.")
    }

    const rejections = result.rejections.map(
      (rejection) => `${rejection.path}: ${rejection.message}`
    )
    const errors = result.diagnostics.map(
      (d) => `${d.path}:${d.line}: ${d.field}: ${d.message}`
    )
    const reasons = [...rejections, ...errors].join("\n")
    return failure(`Core rejected the change. Nothing was applied.\n${reasons}`)
  },
})

export const readTools = [listFiles, readFileTool, searchFiles]
export const writeTools = [...readTools, applyChanges]

export const Workspace = defineExtension({
  name: "workspace",
  tools: writeTools,
})

function workspacePath(path: string) {
  const target = resolve(env.RAY_WORKSPACE, path)
  const inside =
    target === env.RAY_WORKSPACE || target.startsWith(env.RAY_WORKSPACE + sep)
  if (!inside || target.includes(`${sep}.git`)) {
    throw new Error(`${path} is outside the workspace`)
  }

  return target
}

function text(value: string) {
  return { content: [{ type: "text" as const, text: value }] }
}

function failure(value: string) {
  return { ...text(value), isError: true }
}
