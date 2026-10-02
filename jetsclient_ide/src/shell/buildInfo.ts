/**
 * The build information the shell shows at its foot. jetstore_maintenance_02
 * Phase 1, task AC.2, for defect D05.
 *
 * **Two facts, not one twice.** The report asked for *R `<version>` built on
 * YYYY-MM-DD*, and a deployed `JETS_VERSION` *is* the build time — `date +%s`,
 * stamped into every image of a build run — so that text prints one number and
 * its own translation. What the line carries instead is the image's
 * `JETS_GIT_SHA`, which the build stamps as `<branch>-<sha>`, and the date
 * `JETS_VERSION` encodes: *`<branch>-<sha>` · built YYYY-MM-DD* (`Q-1`, answered
 * 2026-10-01). The raw `JETS_VERSION` is the tooltip, so nothing is lost.
 *
 * **`JETS_VERSION` is parsed defensively, because it is not always seconds.** A
 * workstation's run script exports a thirteen-digit value — milliseconds — and
 * CMake falls back to the word `no-version-specified`. So: ten digits are
 * seconds, thirteen are milliseconds, and anything else is shown as it is with
 * no date, rather than turned into a date some fifty thousand years out.
 *
 * **The date is UTC.** A build stamped just after midnight UTC is that day's
 * build everywhere; the viewer's own zone would date it the day before for half
 * the planet.
 */

/** What the footer renders: the visible line and the tooltip. */
export interface BuildInfo {
  /** *`<branch>-<sha>` · built YYYY-MM-DD*, or as much of it as is known. */
  label: string;
  /** The raw `JETS_VERSION`, or `""` when there is none. */
  title: string;
}

/** The separator between the commit and the date. */
const SEP = " · ";

/**
 * `JETS_VERSION` as a UTC calendar date, or `null` when it is not a timestamp.
 *
 * Exactly ten or thirteen digits and nothing else — nine digits of seconds end
 * in September 2001 and eleven begin in 2286, and neither is a build this code
 * has ever produced, so both fall through to the raw string.
 */
export function buildDate(version: string): string | null {
  let millis: number;
  if (/^\d{10}$/.test(version)) millis = Number(version) * 1000;
  else if (/^\d{13}$/.test(version)) millis = Number(version);
  else return null;
  return new Date(millis).toISOString().slice(0, 10);
}

/**
 * Formats the two login values for display, or `null` when there is nothing to
 * show at all.
 *
 * **An empty commit leaves the date alone** — *built 2023-01-06* — which is what
 * a workstation apiserver produces: it sets `JETS_VERSION` and nothing sets
 * `JETS_GIT_SHA`. **A commit with a leading hyphen loses it**: the build script
 * composes the value as `${branch}-${sha}` from `git branch --show-current`,
 * which prints nothing on a detached HEAD, so a build from one is `-6aeb790`, and
 * the sha alone is the honest rendering of that.
 */
export function formatBuildInfo(gitSha: string, version: string): BuildInfo | null {
  const sha = gitSha.trim().replace(/^-+/, "");
  const raw = version.trim();
  const date = buildDate(raw);
  const when = date !== null ? `built ${date}` : raw;
  const label = [sha, when].filter((part) => part !== "").join(SEP);
  if (label === "") return null;
  return { label, title: raw };
}
