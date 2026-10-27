// Loaded before every test file (`setupFiles` in vite.config.ts).
//
// **Testing Library's waits get three seconds rather than one** (jetstore_maintenance_02
// `I-30`, 2026-10-27). `findBy…` and `waitFor` give up after 1 s by default. That is
// generous for one test alone and not for a full parallel run, where every core is busy
// and a case can run four times slower than it does by itself. See the `test` block in
// vite.config.ts for what was measured. A wait that is wrong still fails, three seconds
// later.
import { configure } from "@testing-library/dom";

configure({ asyncUtilTimeout: 3000 });
