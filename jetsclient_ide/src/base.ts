/**
 * Where this app is mounted.
 *
 * **Its own module, and the reason is a cycle rather than tidiness.** It lived in
 * `App.tsx` while `App.tsx` was its only reader; X.4 gave `AppShell` a link that
 * leaves the router's tree — `/register` sits outside it — and importing the
 * constant from `App` would have made `App → AppShell → App`. ESM tolerates that
 * and a test that imports the shell alone does not, so the shared value moves
 * down instead.
 *
 * **This is one of three places the prefix is written and the only one that is
 * TypeScript.** The others are `base` in `vite.config.ts`, which bakes it into
 * every asset url at build time, and `ideAssetPrefix` in
 * `jets/apiserver/static_ide.go`, which serves them. They must agree, and the
 * bundle is not relocatable without a rebuild because of the middle one. Task
 * X.2 (the ui refresh project's I-26) is what makes the three of them say
 * something other than "ide".
 */
export const BASENAME = "/ide";
