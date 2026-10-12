import { createFileRoute } from "@tanstack/react-router"

export const Route = createFileRoute("/chat")({
  head: () => ({ meta: [{ title: "Chat · Prism" }] }),
  component: ChatPage,
})

function ChatPage() {
  return (
    <div className="typeset typeset-chat rounded-xl border bg-card p-4 text-card-foreground">
      <p>
        <strong>Hadrian</strong> was Roman emperor from 117 to 138. You have a
        list named <em>Historical Figures</em>.
      </p>
      <ol>
        <li>Add Hadrian to the list.</li>
        <li>
          Link the page to <a href="#">Trajan</a>.
        </li>
      </ol>
    </div>
  )
}
