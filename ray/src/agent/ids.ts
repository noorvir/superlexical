import type { ConversationId } from "@earendil-works/pi-durable"

/**
 * Reads a conversation ID from client input. Returns nothing when the text is
 * not a valid ID. The ID is not checked against storage.
 */
export function parseConversationId(text: string): ConversationId | undefined {
  const value = Number(text)
  if (!isConversationId(value)) {
    return undefined
  }

  return value
}

function isConversationId(value: number): value is ConversationId {
  return Number.isSafeInteger(value) && value >= 0
}
