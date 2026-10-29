"use client";

// Translations live in frontend/locales/<code>.json. To add a language, copy
// en.json to e.g. de.json, translate the values and add it to LANGUAGES below.
// Texts missing in a language are shown in English (en.json must be complete).

import { createContext, useContext, useEffect, useMemo, type ReactNode } from "react";
import en from "@/locales/en.json";
import sv from "@/locales/sv.json";
import { ApiError } from "./api";

export const LANGUAGES = { en, sv } as const;
export const DEFAULT_LANGUAGE = "en";

export const languageList = Object.entries(LANGUAGES).map(([code, m]) => ({ code, name: m.meta.name }));

type Vars = Record<string, string | number>;
export type T = (key: string, vars?: Vars) => string;

type Tree = { [k: string]: string | Tree };

function lookup(tree: Tree | undefined, key: string): string | undefined {
  let node: string | Tree | undefined = tree;
  for (const part of key.split(".")) {
    if (!node || typeof node === "string") return undefined;
    node = node[part];
  }
  return typeof node === "string" ? node : undefined;
}

const messagesFor = (lang: string) =>
  (LANGUAGES as Record<string, unknown>)[lang] as Tree | undefined;

/** Translate `key` ("section.name") into `lang`, filling in {placeholders}. */
export function translate(lang: string, key: string, vars?: Vars): string {
  return translateWith(messagesFor(lang), key, vars);
}

/** Look `key` up in `messages`, falling back to English, then to the key itself. */
export function translateWith(messages: unknown, key: string, vars?: Vars): string {
  const text = lookup(messages as Tree | undefined, key) ?? lookup(LANGUAGES.en as unknown as Tree, key) ?? key;
  return vars ? text.replace(/\{(\w+)\}/g, (m, name) => (name in vars ? String(vars[name]) : m)) : text;
}

export function hasKey(lang: string, key: string): boolean {
  return lookup(messagesFor(lang), key) !== undefined || lookup(LANGUAGES.en as unknown as Tree, key) !== undefined;
}

/** Text for an error: translated if the backend sent a known code, else the raw message. */
export function errorText(t: T, e: unknown): string {
  if (e instanceof ApiError && e.code) {
    const key = `errors.${e.code}`;
    const text = t(key);
    if (text !== key) return text;
  }
  return e instanceof Error ? e.message : String(e);
}

interface I18n {
  lang: string;
  /** BCP 47 locale for dates/numbers, e.g. "sv-SE" */
  locale: string;
  t: T;
}

const make = (lang: string): I18n => {
  const code = messagesFor(lang) ? lang : DEFAULT_LANGUAGE;
  return {
    lang: code,
    locale: translate(code, "meta.locale"),
    t: (key, vars) => translate(code, key, vars),
  };
};

const I18nContext = createContext<I18n>(make(DEFAULT_LANGUAGE));

export function I18nProvider({ language, children }: { language: string; children: ReactNode }) {
  const value = useMemo(() => make(language), [language]);
  useEffect(() => {
    document.documentElement.lang = value.lang;
  }, [value.lang]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export const useI18n = () => useContext(I18nContext);
