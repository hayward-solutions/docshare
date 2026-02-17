import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'standalone',
  env: {
    NEXT_PUBLIC_BACKEND_URL: process.env.BACKEND_URL || '/api',
    NEXT_PUBLIC_FRONTEND_URL: process.env.FRONTEND_URL || '',
  },
};

export default nextConfig;
