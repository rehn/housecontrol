import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { afterAll, describe, expect, it } from "vitest";
import { ApiError } from "./api";
import { errorText, LANGUAGES, languageList, translate, translateWith } from "./i18n";

type Tree = { [k: string]: string | Tree };

function keys(tree: Tree, prefix = ""): string[] {
  return Object.entries(tree).flatMap(([k, v]) =>
    typeof v === "string" ? [prefix + k] : keys(v, `${prefix}${k}.`),
  );
}

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) return sourceFiles(p);
    return /\.tsx?$/.test(name) && !/\.test\.tsx?$/.test(name) ? [p] : [];
  });
}

describe("language files", () => {
  const en = keys(LANGUAGES.en as unknown as Tree).sort();

  // en.json is the complete reference. Other languages may leave texts out
  // (those show in English); the run just lists what is missing.
  const missing: string[] = [];
  afterAll(() => {
    if (missing.length) console.warn(`Untranslated texts (shown in English):\n  ${missing.join("\n  ")}`);
  });

  for (const [code, messages] of Object.entries(LANGUAGES)) {
    if (code === "en") continue;
    const own = keys(messages as unknown as Tree);

    it(`${code}.json only uses keys that exist in en.json (catches typos)`, () => {
      expect(own.filter((k) => !en.includes(k))).toEqual([]);
      missing.push(...en.filter((k) => !own.includes(k)).map((k) => `${code}: ${k}`));
    });

    it(`${code}.json keeps the {placeholders} of en.json in the texts it translates`, () => {
      const vars = (s: string) => (s.match(/\{\w+\}/g) ?? []).sort();
      for (const k of own.filter((k) => en.includes(k))) {
        expect([k, ...vars(translate(code, k))]).toEqual([k, ...vars(translate("en", k))]);
      }
    });
  }

  it("every t(\"...\") key used in the code exists in en.json", () => {
    const root = join(__dirname, "..");
    const used = new Set<string>();
    for (const dir of ["app", "components", "lib"]) {
      for (const f of sourceFiles(join(root, dir))) {
        for (const m of readFileSync(f, "utf8").matchAll(/\bt\(\s*["']([\w.]+)["']/g)) used.add(m[1]);
      }
    }
    expect(used.size).toBeGreaterThan(20);
    expect([...used].filter((k) => !en.includes(k))).toEqual([]);
  });

  it("lists the languages with their own names", () => {
    expect(languageList).toEqual(
      expect.arrayContaining([
        { code: "en", name: "English" },
        { code: "sv", name: "Svenska" },
      ]),
    );
  });
});

describe("translate", () => {
  it("fills in {placeholders}", () => {
    expect(translate("en", "tile.groupOn", { on: 1, total: 3 })).toBe("1 of 3 on");
    expect(translate("sv", "tile.groupOn", { on: 1, total: 3 })).toBe("1 av 3 på");
  });

  it("falls back to English for unknown languages", () => {
    expect(translate("xx", "common.save")).toBe("Save");
  });

  it("falls back to English for a text missing in the chosen language", () => {
    const partial = { common: { cancel: "Avbryt" } };
    expect(translateWith(partial, "common.cancel")).toBe("Avbryt");
    expect(translateWith(partial, "common.save")).toBe("Save");
    expect(translateWith(partial, "tile.groupOn", { on: 2, total: 3 })).toBe("2 of 3 on");
  });

  it("returns the key itself if it does not exist", () => {
    expect(translate("en", "no.such.key")).toBe("no.such.key");
  });
});

describe("errorText", () => {
  const t = (k: string, v?: Record<string, string | number>) => translate("sv", k, v);

  it("translates known backend error codes", () => {
    expect(errorText(t, new ApiError("not connected to the MQTT broker", "mqtt_not_connected"))).toBe(
      "Inte ansluten till MQTT-brokern",
    );
  });

  it("shows the raw message for errors without a known code", () => {
    expect(errorText(t, new Error("connection refused"))).toBe("connection refused");
    expect(errorText(t, new ApiError("odd", "some_new_code"))).toBe("odd");
  });
});
