import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'standalone',
  env: {
    NEXT_PUBLIC_API_URL: process.env.API_URL || '/api',
    NEXT_PUBLIC_WEB_URL: process.env.WEB_URL || '',
  },
};

export default nextConfig;
