import { defineConfig } from "vite";

// The workspace validator as one Node script.
//
// `npm run build:validator` writes `dist-validator/jets_validate_workspace.mjs`:
// `src/userflow/validateWorkspaceCli.ts` and everything it imports, zod and the
// production escape registry included, in a single ES module with no
// `node_modules` beside it. That is the property the images need — the script
// is copied next to the binaries and run as `node <script> <workspace>`, where
// there is no package to resolve against.
//
// **Vite's SSR build rather than a new bundler.** Vite is already the package's
// build and ships its bundler (Rolldown since vite 8, Rollup and esbuild before),
// so this is a second configuration of the tool that is there, not a dependency.
// `ssr` targets Node — `node:fs` stays a built-in import — and `noExternal: true`
// is what turns "a server build that requires its node_modules" into "one file".
//
// **A second build, and the last step of `npm run build`** (`package.json`), so a
// workstation gets the script beside the browser app without a second command;
// `npm run build:validator` builds it alone. It is a separate configuration
// because the browser app is served as static files and the validator is never
// served. `Dockerfile.cpipes_builder` runs `build:validator` alone and copies
// the script beside the binaries.
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
    // `rolldownOptions` and `codeSplitting: false` are vite 8's names for what were
    // `rollupOptions` and `inlineDynamicImports: true`; the old ones still work, with a warning.
    rolldownOptions: {
      output: {
        format: "es",
        entryFileNames: "jets_validate_workspace.mjs",
        // One file: anything that would be a shared chunk is inlined.
        codeSplitting: false,
      },
    },
  },
  ssr: {
    noExternal: true,
    target: "node",
  },
});
