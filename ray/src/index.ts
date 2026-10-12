import { harness } from "./agent/harness.ts"
import { env } from "./env.ts"
import { startServer } from "./server.ts"

harness.resume()
startServer()

console.log(`Ray listening on http://${env.RAY_HOST}:${env.RAY_PORT}`)
console.log(`Ray uses the model ${env.RAY_MODEL}`)
