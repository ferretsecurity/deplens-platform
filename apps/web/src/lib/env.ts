import { z } from "zod";

const serverEnvSchema = z.object({
  API_INTERNAL_BASE_URL: z.string().url().default("http://localhost:8080"),
  NEXT_PUBLIC_APP_BASE_URL: z.string().url().default("http://localhost:3000")
});

const parsedServerEnv = serverEnvSchema.parse({
  API_INTERNAL_BASE_URL: process.env.API_INTERNAL_BASE_URL,
  NEXT_PUBLIC_APP_BASE_URL: process.env.NEXT_PUBLIC_APP_BASE_URL
});

export const env = parsedServerEnv;
