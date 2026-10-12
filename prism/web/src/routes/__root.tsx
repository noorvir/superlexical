import {
  HeadContent,
  Link,
  Outlet,
  Scripts,
  createRootRoute,
} from "@tanstack/react-router"

import { ServiceStatus } from "@/components/service-status"
import appCss from "../styles.css?url"

// Runs before the first paint so that the page does not show the light theme
// to a person whose system uses the dark theme.
const applySystemTheme = `
const systemTheme = window.matchMedia("(prefers-color-scheme: dark)")
const apply = () => document.documentElement.classList.toggle("dark", systemTheme.matches)
systemTheme.addEventListener("change", apply)
apply()
`

export const Route = createRootRoute({
  head: () => ({
    meta: [
      { charSet: "utf-8" },
      { name: "viewport", content: "width=device-width, initial-scale=1" },
      { title: "Prism" },
    ],
    links: [{ rel: "stylesheet", href: appCss }],
    scripts: [{ children: applySystemTheme }],
  }),
  shellComponent: RootDocument,
  component: Root,
  notFoundComponent: NotFound,
})

function RootDocument({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <HeadContent />
      </head>
      <body>
        {children}
        <Scripts />
      </body>
    </html>
  )
}

function Root() {
  return (
    <div className="flex min-h-svh flex-col">
      <a
        href="#content"
        className="sr-only focus-visible:not-sr-only focus-visible:absolute focus-visible:top-2 focus-visible:left-2 focus-visible:rounded-md focus-visible:bg-background focus-visible:px-3 focus-visible:py-2 focus-visible:text-sm focus-visible:ring-2 focus-visible:ring-ring"
      >
        Skip to content
      </a>
      <header className="flex h-12 shrink-0 items-center gap-6 border-b px-4 sm:px-6">
        <span className="text-sm font-semibold">Prism</span>
        <nav className="flex flex-1 items-center gap-1">
          <NavLink to="/">Wiki</NavLink>
          <NavLink to="/chat">Chat</NavLink>
        </nav>
        <ServiceStatus />
      </header>
      <main
        id="content"
        className="mx-auto w-full max-w-2xl flex-1 px-4 py-10 sm:px-6"
      >
        <Outlet />
      </main>
    </div>
  )
}

function NavLink({
  to,
  children,
}: {
  to: "/" | "/chat"
  children: React.ReactNode
}) {
  return (
    <Link
      to={to}
      activeOptions={{ exact: true }}
      className="rounded-md px-2.5 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none aria-[current=page]:bg-accent/50 aria-[current=page]:font-medium aria-[current=page]:text-accent-foreground"
    >
      {children}
    </Link>
  )
}

function NotFound() {
  return (
    <div className="typeset">
      <h1>Page not found</h1>
      <p>
        This page does not exist. <Link to="/">Go to the wiki</Link>.
      </p>
    </div>
  )
}
