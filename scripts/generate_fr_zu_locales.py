#!/usr/bin/env python3
"""Generate fr.json and zu.json from en.json using batched machine translation."""
import json
import re
import sys
import time
from pathlib import Path

from deep_translator import GoogleTranslator

ROOT = Path(__file__).resolve().parents[1]
LOCALES = ROOT / "internal" / "web" / "locales"
EN_PATH = LOCALES / "en.json"
BATCH = 40

SKIP_TRANSLATE = {"app.title", "nav.qwk", "nav.netmail", "pwa.description"}

MANUAL = {
    "fr": {
        "lang.en": "English",
        "lang.es": "Español",
        "lang.af": "Afrikaans",
        "lang.fr": "Français",
        "lang.zu": "isiZulu",
        "nav.qwk": "QWK",
        "nav.netmail": "Netmail",
        "admin_binkp.stat.netmail": "Netmail",
        "admin_binkp.stat.echomail": "Échomail",
    },
    "zu": {
        "lang.en": "English",
        "lang.es": "Español",
        "lang.af": "Afrikaans",
        "lang.fr": "Français",
        "lang.zu": "isiZulu",
        "nav.qwk": "QWK",
        "nav.netmail": "Netmail",
        "admin_binkp.stat.netmail": "Netmail",
        "admin_binkp.stat.echomail": "Echomail",
    },
}

PLACEHOLDER_RE = re.compile(r"(%[0-9]*(?:\.[0-9]+)?[sdif]|%s|%\.\d+f)")


def protect_placeholders(text: str) -> tuple[str, list[str]]:
    tokens: list[str] = []

    def repl(m: re.Match[str]) -> str:
        tokens.append(m.group(0))
        return f"⟦{len(tokens)-1}⟧"

    return PLACEHOLDER_RE.sub(repl, text), tokens


def restore_placeholders(text: str, tokens: list[str]) -> str:
    for i, tok in enumerate(tokens):
        for variant in (f"⟦{i}⟧", f"⟦ {i} ⟧", f"⟦{i} ⟧"):
            text = text.replace(variant, tok)
    return text


def translate_batch(translator: GoogleTranslator, texts: list[str]) -> list[str]:
    for attempt in range(3):
        try:
            return translator.translate_batch(texts)
        except Exception as e:
            print(f"  batch retry {attempt+1}: {e}", flush=True)
            time.sleep(2 * (attempt + 1))
    return texts


def build_locale(target: str) -> dict[str, str]:
    en: dict[str, str] = json.loads(EN_PATH.read_text(encoding="utf-8"))
    translator = GoogleTranslator(source="en", target=target)
    out: dict[str, str] = {}
    pending_keys: list[str] = []
    pending_protected: list[str] = []
    pending_tokens: list[list[str]] = []

    def flush() -> None:
        nonlocal pending_keys, pending_protected, pending_tokens
        if not pending_keys:
            return
        translated = translate_batch(translator, pending_protected)
        for key, text, tokens in zip(pending_keys, translated, pending_tokens):
            out[key] = restore_placeholders(text, tokens)
        print(f"  {target}: {len(out)} keys", flush=True)
        pending_keys, pending_protected, pending_tokens = [], [], []
        time.sleep(0.3)

    for key in sorted(en.keys()):
        val = en[key]
        if key in SKIP_TRANSLATE or key in MANUAL.get(target, {}):
            out[key] = MANUAL.get(target, {}).get(key, val)
            continue
        protected, tokens = protect_placeholders(val)
        pending_keys.append(key)
        pending_protected.append(protected)
        pending_tokens.append(tokens)
        if len(pending_keys) >= BATCH:
            flush()
    flush()
    out.update(MANUAL.get(target, {}))
    return out


def main() -> None:
    targets = sys.argv[1:] or ["fr", "zu"]
    for target in targets:
        print(f"Building {target}.json ...", flush=True)
        data = build_locale(target)
        path = LOCALES / f"{target}.json"
        path.write_text(
            json.dumps(data, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        print(f"Wrote {path} ({len(data)} keys)", flush=True)


if __name__ == "__main__":
    main()
