import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    const apiBaseUrl = process.env.API_INTERNAL_BASE_URL ?? "http://localhost:8080";
    return [
      { source: "/auth/:path*", destination: `${apiBaseUrl}/auth/:path*` },
      { source: "/api/:path*", destination: `${apiBaseUrl}/api/:path*` }
    ];
  }
};

export default nextConfig;
