/**
 * The build line's formatter. jetstore_maintenance_02 Phase 1, task AC.2, for
 * D05 — criterion 6: *seconds, milliseconds, empty and non-numeric versions each
 * render as specified*.
 *
 * The inputs are the ones the deployments actually produce, not invented edge
 * cases: `date +%s` from the build script, the thirteen-digit value a
 * workstation's run script exports, an apiserver with nothing set, and CMake's
 * `no-version-specified` fallback.
 */

import { afterEach, describe, expect, it } from "vitest";

import { buildDate, formatBuildInfo } from "./buildInfo";

const SHA = "jets_ai-6aeb79068";

describe("formatBuildInfo", () => {
  it("renders a deployed build: ten digits are seconds", () => {
    // 1759338000 is 2025-10-01T17:00:00Z.
    expect(formatBuildInfo(SHA, "1759338000")).toEqual({
      label: "jets_ai-6aeb79068 · built 2025-10-01",
      title: "1759338000",
    });
  });

  it("renders a workstation build: thirteen digits are milliseconds", () => {
    // `internal/run_env.sh`'s value; 2023-01-06T02:48:30.850Z. Read as seconds
    // it would be a date some fifty thousand years out.
    expect(formatBuildInfo(SHA, "1672973310850")).toEqual({
      label: "jets_ai-6aeb79068 · built 2023-01-06",
      title: "1672973310850",
    });
  });

  it("shows an empty version as nothing, with no date and no separator", () => {
    expect(formatBuildInfo(SHA, "")).toEqual({ label: SHA, title: "" });
  });

  it("shows a non-numeric version raw, without a date", () => {
    expect(formatBuildInfo(SHA, "no-version-specified")).toEqual({
      label: "jets_ai-6aeb79068 · no-version-specified",
      title: "no-version-specified",
    });
  });

  it("shows digit runs of the wrong length raw rather than as a date", () => {
    // Nine digits of seconds end in 2001 and eleven begin in 2286; twelve and
    // fourteen are neither unit. None is a build anybody made.
    for (const v of ["999999999", "17593380000", "175933800000", "17593380000000"]) {
      expect(formatBuildInfo(SHA, v)?.label).toBe(`${SHA} · ${v}`);
    }
  });

  it("does not read a number with anything around it as a timestamp", () => {
    expect(formatBuildInfo(SHA, "v1759338000")?.label).toBe(`${SHA} · v1759338000`);
    expect(formatBuildInfo(SHA, "1759338000.5")?.label).toBe(`${SHA} · 1759338000.5`);
  });

  it("trims surrounding whitespace from both values", () => {
    expect(formatBuildInfo(` ${SHA}\n`, " 1759338000\n")).toEqual({
      label: "jets_ai-6aeb79068 · built 2025-10-01",
      title: "1759338000",
    });
  });

  describe("with no commit", () => {
    it("leaves the date alone: a workstation apiserver sets no JETS_GIT_SHA", () => {
      expect(formatBuildInfo("", "1672973310850")).toEqual({
        label: "built 2023-01-06",
        title: "1672973310850",
      });
    });

    it("leaves a raw version alone", () => {
      expect(formatBuildInfo("", "no-version-specified")?.label).toBe("no-version-specified");
    });

    it("is null when there is nothing to show at all, so no footer is drawn", () => {
      expect(formatBuildInfo("", "")).toBeNull();
      expect(formatBuildInfo("  ", " ")).toBeNull();
    });
  });

  it("drops the hyphen a build from a detached HEAD leaves at the front", () => {
    // `${branch}-${sha}` with `git branch --show-current` printing nothing.
    expect(formatBuildInfo("-6aeb790", "1759338000")?.label).toBe(
      "6aeb790 · built 2025-10-01",
    );
    expect(formatBuildInfo("-", "1759338000")?.label).toBe("built 2025-10-01");
  });
});

describe("buildDate is UTC, whatever the viewer's zone", () => {
  const original = process.env["TZ"];
  afterEach(() => {
    if (original === undefined) delete process.env["TZ"];
    else process.env["TZ"] = original;
  });

  it("dates a build stamped at midnight UTC that day, seen from New York", () => {
    // 1759363200 is 2025-10-02T00:00:00Z, which is 20:00 on 2025-10-01 in New
    // York: a formatter using local fields dates it a day early. Node re-reads
    // TZ on assignment, so the local zone really is New York for this case.
    process.env["TZ"] = "America/New_York";
    expect(new Date(1759363200 * 1000).getDate()).toBe(1); // the zone took
    expect(buildDate("1759363200")).toBe("2025-10-02");
    expect(buildDate("1759363200500")).toBe("2025-10-02");
  });

  it("dates the last second before midnight UTC that day, seen from Tokyo", () => {
    // 2025-10-01T23:59:59Z is 08:59 on 2025-10-02 in Tokyo.
    process.env["TZ"] = "Asia/Tokyo";
    expect(new Date(1759363199 * 1000).getDate()).toBe(2); // the zone took
    expect(buildDate("1759363199")).toBe("2025-10-01");
  });

  it("is null for anything that is not ten or thirteen digits", () => {
    for (const v of ["", "no-version-specified", "175933800", "17593380000", "-1759338000"]) {
      expect(buildDate(v)).toBeNull();
    }
  });
});
