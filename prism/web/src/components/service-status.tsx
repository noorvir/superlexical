import { useQuery } from "@tanstack/react-query"

import { getCoreHealth, getDiagnostics, getRayHealth } from "@/lib/api"

const refetchInterval = 5000

export function ServiceStatus() {
  const core = useQuery({
    queryKey: ["core", "health"],
    queryFn: getCoreHealth,
    refetchInterval,
    retry: false,
  })
  const ray = useQuery({
    queryKey: ["ray", "health"],
    queryFn: getRayHealth,
    refetchInterval,
    retry: false,
  })
  const diagnostics = useQuery({
    queryKey: ["core", "diagnostics"],
    queryFn: getDiagnostics,
    refetchInterval,
    retry: false,
  })

  let workspace = "Not checked"
  if (diagnostics.isSuccess) {
    workspace = "Valid"
  }
  if (diagnostics.isSuccess && diagnostics.data.length > 0) {
    workspace = `${diagnostics.data.length} errors`
  }

  return (
    <dl className="flex items-center gap-5 text-xs">
      <Service name="Core" status={core.status} />
      <Service name="Ray" status={ray.status} />
      <div className="hidden items-center gap-1.5 sm:flex">
        <dt className="text-muted-foreground">Workspace</dt>
        <dd className="font-medium">{workspace}</dd>
      </div>
    </dl>
  )
}

const labels = {
  pending: "Checking",
  success: "Running",
  error: "Not reachable",
}

function Service({
  name,
  status,
}: {
  name: string
  status: keyof typeof labels
}) {
  return (
    <div className="flex items-center gap-1.5">
      <dt className="text-muted-foreground">{name}</dt>
      <dd
        data-status={status}
        className="font-medium data-[status=error]:text-destructive"
      >
        {labels[status]}
      </dd>
    </div>
  )
}
