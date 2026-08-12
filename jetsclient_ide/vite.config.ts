import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The bundle is served by the Go apiserver under /ide/ (see jets/apiserver/server.go),
// so every emitted asset url has to carry that prefix. In dev the same endpoints are
// proxied to a locally running apiserver, which keeps the client's origin handling
// identical in both modes — it always talks to a same-origin path.
const API_PATHS = ["/dataTable", "/login", "/register", "/inferServer", "/registerFileKey", "/purgeData"];

// The apiserver listens on :8080 with -usingSshTunnel and on :8443 (TLS) otherwise,
// so the dev target is configurable. `secure: false` matters for the :8443 case,
// where the certificate is self-signed.
const apiOrigin = process.env["JETS_API_ORIGIN"] ?? "http://localhost:8080";

export default defineConfig({
  base: "/ide/",
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
    include: ["src/**/*.test.ts"],
  },
});
