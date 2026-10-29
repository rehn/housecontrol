import { describe, expect, it } from "vitest";
import { describeDays, describeTrigger, describeNext, WEEK } from "./schedules";
import { translate } from "./i18n";

const en = (key: string, vars?: Record<string, string | number>) => translate("en", key, vars);
const sv = (key: string, vars?: Record<string, string | number>) => translate("sv", key, vars);

describe("describeTrigger", () => {
  it("fixed time", () => {
    expect(describeTrigger({ kind: "time", time: "07:00" }, en)).toBe("07:00");
  });

  it("sun with offset and limits", () => {
    expect(describeTrigger({ kind: "sunset" }, en)).toBe("at sunset");
    expect(describeTrigger({ kind: "sunset", offset: -30 }, en)).toBe("30 min before sunset");
    expect(describeTrigger({ kind: "sunrise", offset: 15 }, en)).toBe("15 min after sunrise");
    expect(describeTrigger({ kind: "sunset", offset: -30, notBefore: "16:00" }, en)).toBe(
      "30 min before sunset, not before 16:00",
    );
    expect(describeTrigger({ kind: "sunrise", notAfter: "08:00" }, en)).toBe("at sunrise, not after 08:00");
  });

  it("in Swedish", () => {
    expect(describeTrigger({ kind: "sunset", offset: -30, notBefore: "16:00" }, sv)).toBe(
      "30 min före solnedgång, tidigast 16:00",
    );
    expect(describeTrigger({ kind: "sunrise" }, sv)).toBe("vid soluppgång");
  });
});

describe("describeDays", () => {
  it("names common sets of days", () => {
    expect(describeDays([], en, "en")).toBe("Every day");
    expect(describeDays([1, 2, 3, 4, 5], en, "en")).toBe("Weekdays");
    expect(describeDays([0, 6], en, "en")).toBe("Weekends");
    expect(describeDays([5, 1, 3], sv, "sv")).toBe("mån, ons, fre");
  });

  it("starts the week on Monday", () => {
    expect(WEEK).toEqual([1, 2, 3, 4, 5, 6, 0]);
  });
});

describe("describeNext", () => {
  const now = new Date("2026-10-29T12:00:00+01:00"); // Thursday
  it("says today / tomorrow / weekday", () => {
    expect(describeNext({ action: "on", at: "2026-10-29T16:12:00+01:00" }, now, en, "en")).toBe("turns on today 16:12");
    expect(describeNext({ action: "off", at: "2026-10-30T07:00:00+01:00" }, now, en, "en")).toBe("turns off tomorrow 07:00");
    expect(describeNext({ action: "on", at: "2026-11-02T06:30:00+01:00" }, now, sv, "sv")).toBe("tänds mån 06:30");
  });
});
