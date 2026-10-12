import { createFileRoute } from "@tanstack/react-router"

export const Route = createFileRoute("/")({
  head: () => ({ meta: [{ title: "Hadrian · Prism" }] }),
  component: WikiPage,
})

function WikiPage() {
  return (
    <article className="typeset typeset-docs">
      <h1>Hadrian</h1>
      <p>
        Hadrian was Roman emperor from 117 to 138. He spent more than half of
        his reign outside Italy, and he chose to secure the borders of the
        empire instead of extending them. See also <a href="#">Trajan</a> and{" "}
        <a href="#">Antinous</a>.
      </p>
      <h2>Building works</h2>
      <p>
        His projects appear in every province. The best known are the{" "}
        <strong>Pantheon</strong> in Rome, his villa at Tivoli, and the wall
        across the north of Britain.
      </p>
      <ul>
        <li>Pantheon, Rome</li>
        <li>Villa Adriana, Tivoli</li>
        <li>
          Hadrian&apos;s Wall, Britannia
          <ul>
            <li>Length: 117 kilometres</li>
          </ul>
        </li>
      </ul>
      <blockquote>
        <p>Little soul, gentle and wandering, guest and friend of the body.</p>
      </blockquote>
      <h3>Record</h3>
      <p>
        Each page is a Markdown file. Its front matter holds the{" "}
        <code>@id</code> and <code>@type</code> of the record.
      </p>
      <pre>
        <code>{`---
context": https://schema.org
id": person:zt507zg4MOyd_2TGcpFB0
type": Person
me: Hadrian
-`}</code>
      </pre>
      <table>
        <thead>
          <tr>
            <th>Emperor</th>
            <th>Reign</th>
            <th align="right">Years</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>Trajan</td>
            <td>98 to 117</td>
            <td align="right">19</td>
          </tr>
          <tr>
            <td>Hadrian</td>
            <td>117 to 138</td>
            <td align="right">21</td>
          </tr>
        </tbody>
      </table>
      <hr />
      <h6>Lists</h6>
      <p>Historical Figures</p>
    </article>
  )
}
