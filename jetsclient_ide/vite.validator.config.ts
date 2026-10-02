import { defineConfig } from "vite";

// The workspace validator as one Node script — `jetstore_maintenance_02` AG.3.
//
// `npm run build:validator` writes `dist-validator/jets_validate_workspace.mjs`:
// `src/userflow/validateWorkspaceCli.ts` and everything it imports, zod and the
// production escape registry included, in a single ES module with no
// `node_modules` beside it. That is the property the images need — the script
// is copied next to the binaries and run as `node <script> <workspace>`, where
// there is no package to resolve against.
//
// **Vite's SSR build rather than a new bundler.** Vite is already the package's
// build and ships Rollup and esbuild, so this is a second configuration of the
// tool that is there, not a dependency. `ssr` targets Node — `node:fs` stays a
// built-in import — and `noExternal: true` is what turns "a server build that
// requires its node_modules" into "one file".
//
// **Not part of `npm run build`**, which is the browser app and is served as
// static files; the validator is never served. The Docker build runs both.
export default defineConfig({
  // The app's favicons are for the browser build; nothing here is served.
  publicDir: false,
  build: {
    ssr: "src/userflow/validateWorkspaceCli.ts",
    outDir: "dist-validator",
    emptyOutDir: true,
    target: "node22",
    sourcemap: false,
    minify: false,
    rollupOptions: {
      output: {
        format: "es",
        entryFileNames: "jets_validate_workspace.mjs",
        // One file: anything that would be a shared chunk is inlined.
        inlineDynamicImports: true,
      },
    },
  },
  ssr: {
    noExternal: true,
    target: "node",
  },
});
