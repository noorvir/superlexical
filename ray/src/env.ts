export const env = {
  RAY_HOST: process.env.RAY_HOST ?? "127.0.0.1",
  RAY_PORT: Number(process.env.RAY_PORT ?? "4200"),
}
