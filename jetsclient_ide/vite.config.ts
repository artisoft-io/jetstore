/// <reference types="vitest/config" />
// vitest 4 no longer adds the `test` key below to vite's config type on its own; this reference does.
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
  // **The budgets are for a full parallel run, not for one file** (jetstore_maintenance_02
  // `I-30`, 2026-10-27). Twenty consecutive `npx vitest run` on a 16-core machine failed 3
  // times. Two of the failures were races, fixed where they were (`widgets/Dropdown.tsx`,
  // `ProcessErrors.test.tsx`). The rest were budgets: cases that take 0.2 s to 3.4 s alone
  // ran past the 1 s wait default or the 5 s test default in a run that took 17 s rather
  // than about 10. With the two races fixed and those cases given budgets of their own, a
  // third case, 1.3 s alone, timed out the same way in the next sixteen runs. So the budget
  // is suite-wide: 15 s per test here, and 3 s per wait in vitest.setup.ts.
  test: {
    environment: "node",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    setupFiles: ["./vitest.setup.ts"],
    testTimeout: 15_000,
  },
});
