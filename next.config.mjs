// Next serves the UI; all chat and commerce logic lives in Go.
export default {
  experimental: { proxyTimeout: 300_000 }, // Let Go's four-minute turn timeout decide.
  async rewrites() {
    return [{ source: '/api/:path*', destination: 'http://127.0.0.1:8080/api/:path*' }];
  }
};
