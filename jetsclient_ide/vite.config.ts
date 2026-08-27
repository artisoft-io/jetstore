import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The bundle is served by the Go apiserver at the root (see `appAssetPrefix` in
// jets/apiserver/static_ide.go), and every emitted asset url carries that prefix.
// In dev the api endpoints below are proxied to a locally running apiserver, which
// keeps the client's origin handling identical in both modes — it always talks to
// a same-origin path.
//
// **`base` was "/ide/" until X.2**, and it is the reason the rename was a rebuild
// rather than a move: vite writes this value into every script and stylesheet url
// in index.html at build time, so a bundle built for one prefix cannot be served
// from another. The three places that must agree are here, `appAssetPrefix` in
// Go, and `BASENAME` in src/base.ts.
const API_PATHS = ["/dataTable", "/login", "/register", "/inferServer", "/registerFileKey", "/purgeData", "/agentic"];

// The apiserver listens on :8080 with -usingSshTunnel and on :8443 (TLS) otherwise,
// so the dev target is configurable. `secure: false` matters for the :8443 case,
// where the certificate is self-signed.
const apiOrigin = process.env["JETS_API_ORIGIN"] ?? "http://localhost:8080";

export default defineConfig({
  base: "/",
  plugins: [react()],
  build: {
    outDir: "dist",
    // The Go handler serves whatever is here; hashed names are why the old
    // per-file route list could not have worked.
    assetsDir: "assets",
    sourcemap: true,
  },
  server: {
    port: 5173,
    proxy: Object.fromEntries(
      API_PATHS.map((p) => [p, { target: apiOrigin, changeOrigin: true, secure: false }]),
    ),
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
  },
});
