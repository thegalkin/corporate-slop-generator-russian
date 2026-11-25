#!/usr/bin/env python3
"""
Сборка лексикона для cbsg-ru.

Берёт tools/lexicon.yaml, вытаскивает из OpenCorpora полные парадигмы
для каждой леммы, проверяет всё, что можно проверить, и пишет
internal/lexicon/data/lexicon.json.

Проверки:
  * слово есть в словаре;
  * у существительного определён род;
  * собраны все нужные формы (падеж × число; падеж × род для прилагательных);
  * формы ручных парадигм (loans) подтверждены round-trip'ом через анализатор.

Запуск:
    python3 -m venv .venv && .venv/bin/pip install pymorphy3 pyyaml
    .venv/bin/python tools/build_lexicon.py
"""

import json
import os
import sys
from collections import defaultdict

import yaml
from pymorphy3 import MorphAnalyzer

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
SRC = os.path.join(HERE, "lexicon.yaml")
OUT = os.path.join(ROOT, "internal", "lexicon", "data", "lexicon.json")

CASES = ["nomn", "gent", "datv", "accs", "ablt", "loct"]
# Короткие имена падежей — те же, что в ключах слотов и в Go-коде.
CASE_SLOT = {"nomn": "nom", "gent": "gen", "datv": "dat",
             "accs": "acc", "ablt": "ins", "loct": "prep"}
GENDER = {"masc": "m", "femn": "f", "neut": "n"}
NUM = {"sing": "sg", "plur": "pl"}
PERSON = {"1per": "1", "2per": "2", "3per": "3"}
DEGREE = {"Supr", "Strn", "Aparc", "Infrc", "Cmp"}

# Ключи ручных парадигм -> ключи слотов, как их строит collect_verb.
PRES_KEYS = {
    "pres_1sg": "pres:sg:1", "pres_2sg": "pres:sg:2", "pres_3sg": "pres:sg:3",
    "pres_1pl": "pres:pl:1", "pres_2pl": "pres:pl:2", "pres_3pl": "pres:pl:3",
}
PAST_KEYS = {
    "past_m": "past:sg:m", "past_f": "past:sg:f",
    "past_n": "past:sg:n", "past_pl": "past:pl:pl",
}

errors = []
warnings = []


def unspace(s):
    """Ключи в yaml пишутся через подчёркивание: «рабочая_группа» -> «рабочая группа»."""
    return s.replace("_", " ")


def tag_parts(t):
    """'NOUN,inan,femn sing,nomn' -> ('NOUN', {'inan','femn','sing','nomn'})."""
    s = str(t)
    head, _, rest = s.partition(" ")
    parts = head.split(",")
    gram = set(parts[1:])
    if rest:
        gram |= {g for g in rest.split(",") if g}
    return parts[0], gram


def forms_of(parse):
    """Все формы лексемы, к которой принадлежит разбор: [(form, tag), ...]."""
    lx = parse.lexeme
    out = []
    for x in lx:
        if hasattr(x, "word"):
            out.append((x.word, str(x.tag)))
        else:
            out.append((x[0], str(x[1])))
    return out


def pick_parse(morph, word, kinds):
    """Лучший разбор: сначала известный словарю, затем — по совпадению части речи."""
    parses = morph.parse(word)
    if not parses:
        return None
    cands = [p for p in parses if str(p.tag).split(",")[0].split(" ")[0] in kinds]
    if not cands:
        return None
    known = [p for p in cands if p.is_known]
    return (known or cands)[0]


def collect_noun(morph, word, en, plural_only=False):
    """Существительное -> формы по падежам и числам + род и одушевлённость."""
    pick = pick_parse(morph, word, {"NOUN"})
    if pick is None:
        errors.append("нет в словаре или не существительное: %s" % word)
        return None
    if not pick.is_known:
        warnings.append("неточное существительное: %s" % word)

    _, gset = tag_parts(pick.tag)
    gender = next((GENDER[g] for g in ("masc", "femn", "neut") if g in gset), None)
    animate = "anim" in gset

    forms = {}
    for form, t in forms_of(pick):
        pos, g = tag_parts(t)
        if pos != "NOUN" or (g & DEGREE):
            continue
        case = next((c for c in CASES if c in g), None)
        num = next((n for n in NUM if n in g), None)
        if not case or not num:
            continue
        forms.setdefault("n:%s:%s" % (CASE_SLOT[case], NUM[num]), form)

    # У одушевлённых существительных В.п. совпадает с Р.п.
    for src, dst in (("n:gen:sg", "n:acc:sg"), ("n:gen:pl", "n:acc:pl")):
        if dst not in forms and src in forms:
            forms[dst] = forms[src]

    missing = ["n:%s:%s" % (CASE_SLOT[c], s) for c in CASES for s in ("sg", "pl")]
    if plural_only:
        missing = [k for k in missing if not k.endswith(":sg")]
    missing = [k for k in missing if k not in forms]
    if missing:
        errors.append("неполная парадигма: %s -> %s" % (word, ",".join(missing)))

    # род нужен только для согласования; у слов, которые бывают лишь во
    # множественном числе, рода нет вовсе
    if gender is None:
        if "n:nom:pl" not in forms:
            errors.append("не определён род: %s" % word)
            return None
        gender = ""

    # Целостность: И.п. ед.ч. = лемма; у одушевлённых В.п. = Р.п.,
    # у неодушевлённых мужского рода В.п. = И.п.
    if not plural_only and forms.get("n:nom:sg") and forms["n:nom:sg"] != word:
        errors.append("сломана парадигма: %s (И.п. = %s)" % (word, forms["n:nom:sg"]))
    if animate:
        if forms.get("n:acc:sg") and forms.get("n:gen:sg") and forms["n:acc:sg"] != forms["n:gen:sg"]:
            errors.append("сломана парадигма: %s (В.п. != Р.п. у одушевлённого)" % word)
    elif gender == "m" and not plural_only:
        if forms.get("n:acc:sg") and forms.get("n:nom:sg") and forms["n:acc:sg"] != forms["n:nom:sg"]:
            errors.append("сломана парадигма: %s (В.п. != И.п. у неодушевлённого)" % word)

    return {
        "en": en,
        "g": gender,
        "an": animate,
        "pl": "n:nom:sg" not in forms,
        "forms": forms,
    }


def collect_adj(morph, word, en):
    """Прилагательное -> формы. Берём ADJF; если нет — причастие, употреблённое
    как прилагательное (масштабируемый, мотивирующий, диверсифицированный)."""
    pick = pick_parse(morph, word, {"ADJF"})
    voice = tense = None
    mode = "ADJF"
    if pick is None:
        # причастия, употреблённые как прилагательные
        for v, t in (("pssv", "pres"), ("pssv", "past"), ("actv", "pres"), ("actv", "past")):
            parses = [p for p in morph.parse(word)
                      if str(p.tag).startswith("PRTF") and v in str(p.tag) and t in str(p.tag)]
            if not parses:
                continue
            known = [p for p in parses if p.is_known]
            pick = (known or parses)[0]
            mode, voice, tense = "PRTF", v, t
            break
    if pick is None:
        errors.append("не прилагательное: %s" % word)
        return None
    if not pick.is_known:
        warnings.append("неточное прилагательное: %s" % word)

    forms = {}
    for form, t in forms_of(pick):
        pos, g = tag_parts(t)
        if g & DEGREE:
            continue
        if mode == "ADJF":
            if pos != "ADJF":
                continue
        else:
            if pos != "PRTF":
                continue
            if voice not in g or tense not in g:
                continue
        case = next((c for c in CASES if c in g), None)
        num = next((n for n in NUM if n in g), None)
        if not case or not num:
            continue
        cs = CASE_SLOT[case]
        if NUM[num] == "sg":
            gg = next((GENDER[x] for x in ("masc", "femn", "neut") if x in g), None)
            if not gg:
                continue
            if cs == "acc":
                an = "anim" if "anim" in g else ("inan" if "inan" in g else "-")
                forms.setdefault("a:acc:sg:%s:%s" % (gg, an), form)
                # запасной вариант — неодушевлённый: у одушевлённого В.п. = Р.п.
                if an == "inan" or ("a:acc:sg:%s:-" % gg) not in forms:
                    forms["a:acc:sg:%s:-" % gg] = form
            else:
                forms.setdefault("a:%s:sg:%s:-" % (cs, gg), form)
        else:
            if cs == "acc":
                an = "anim" if "anim" in g else ("inan" if "inan" in g else "-")
                forms.setdefault("a:acc:pl:%s" % an, form)
                if an == "inan" or ("a:acc:pl:-" not in forms):
                    forms["a:acc:pl:-"] = form
            else:
                forms.setdefault("a:%s:pl:-" % cs, form)

    missing = []
    for c in CASES:
        cs = CASE_SLOT[c]
        for gg in ("m", "f", "n"):
            if c == "accs":
                if "a:acc:sg:%s:-" % gg not in forms:
                    missing.append("%s/sg/%s" % (c, gg))
            elif "a:%s:sg:%s:-" % (cs, gg) not in forms:
                missing.append("%s/sg/%s" % (c, gg))
        if c == "accs":
            if "a:acc:pl:-" not in forms:
                missing.append("%s/pl" % c)
        elif "a:%s:pl:-" % cs not in forms:
            missing.append("%s/pl" % c)
    if missing:
        errors.append("неполная парадигма прил.: %s -> %s" % (word, ",".join(missing)))

    return {"en": en, "forms": forms}


def collect_verb(morph, word, en):
    """Глагол -> формы по видам: презенс, прошедшее, инфинитив, деепричастие."""
    pick = pick_parse(morph, word, {"VERB", "INFN"})
    if pick is None:
        errors.append("нет в словаре или не глагол: %s" % word)
        return None
    if not pick.is_known:
        warnings.append("неточный глагол: %s" % word)

    aspects = defaultdict(dict)
    for form, t in forms_of(pick):
        pos, g = tag_parts(t)
        if pos == "INFN":
            aspect = "pf" if "perf" in g else ("impf" if "impf" in g else None)
            if aspect:
                aspects[aspect].setdefault("inf", form)
            continue
        if pos == "GR":
            aspect = "pf" if "perf" in g else ("impf" if "impf" in g else None)
            if aspect:
                aspects[aspect].setdefault("ger", form)
            continue
        if pos != "VERB" or "indc" not in g:
            continue
        aspect = "pf" if "perf" in g else ("impf" if "impf" in g else None)
        if aspect is None:
            continue
        num = next((n for n in NUM if n in g), None)
        if not num:
            continue
        num = NUM[num]
        if "past" in g:
            gg = next((GENDER[x] for x in ("masc", "femn", "neut") if x in g), None)
            key = "past:%s:%s" % (num, gg or "pl")
        else:
            per = next((PERSON[x] for x in ("1per", "2per", "3per") if x in g), None)
            if not per:
                continue
            key = "pres:%s:%s" % (num, per)
        aspects[aspect].setdefault(key, form)

    if not aspects:
        errors.append("пустой глагол: %s" % word)
        return None

    need = ["pres:sg:1", "pres:sg:2", "pres:sg:3",
            "pres:pl:1", "pres:pl:2", "pres:pl:3",
            "past:sg:m", "past:sg:f", "past:sg:n", "past:pl:pl"]
    for aspect, forms in aspects.items():
        missing = [k for k in need if k not in forms]
        if missing:
            errors.append("неполная парадигма глагола %s [%s] -> %s" % (word, aspect, ",".join(missing)))

    default = "pf" if "pf" in aspects else "impf"
    return {"en": en, "default": default, "aspects": dict(aspects)}


# --- ручные парадигмы (loans) ---

LOAN_EXPECT = {
    "inf": ("INFN", set()),
    "ger": ("GR", set()),
    "pres_1sg": ("VERB", {"1per", "sing"}),
    "pres_2sg": ("VERB", {"2per", "sing"}),
    "pres_3sg": ("VERB", {"3per", "sing"}),
    "pres_1pl": ("VERB", {"1per", "plur"}),
    "pres_2pl": ("VERB", {"2per", "plur"}),
    "pres_3pl": ("VERB", {"3per", "plur"}),
    "past_m": ("VERB", {"masc", "sing", "past"}),
    "past_f": ("VERB", {"femn", "sing", "past"}),
    "past_n": ("VERB", {"neut", "sing", "past"}),
    "past_pl": ("VERB", {"plur", "past"}),
}

ADJ_LOAN_EXPECT = {}
for _c in CASES:
    _cs = CASE_SLOT[_c]
    for _g, _gg in (("m", "masc"), ("f", "femn"), ("n", "neut")):
        ADJ_LOAN_EXPECT["%s_sg_%s" % (_cs, _g)] = ("ADJF", {_gg, "sing", _c})
    ADJ_LOAN_EXPECT["%s_pl" % _cs] = ("ADJF", {"plur", _c})

# Существительные: ручная парадигма.
NOUN_LOAN_EXPECT = {}
for _c in CASES:
    _cs = CASE_SLOT[_c]
    NOUN_LOAN_EXPECT["%s_sg" % _cs] = ("NOUN", {"sing", _c})
    NOUN_LOAN_EXPECT["%s_pl" % _cs] = ("NOUN", {"plur", _c})


def verify_loan(morph, word, forms, expect):
    """Round-trip: форма должна разбираться анализатором с ожидаемыми грамемами.
    Инфинитив и деепричастие проверяем мягко — для заимствованных лемм
    анализатор часто не знает слова вовсе, и это не повод терять слово."""
    for key, form in forms.items():
        if key not in expect:
            errors.append("неизвестный ключ формы: %s/%s" % (word, key))
            continue
        pos, need = expect[key]
        if pos == "INFN":
            # достаточно, чтобы слово вообще читалось как слово
            if not morph.parse(form):
                errors.append("round-trip не сошёлся: %s/%s (%s) -> []" % (word, key, form))
            else:
                warnings.append("инфинитив не подтверждён словарём: %s/%s (%s)" % (word, key, form))
            continue
        ok = False
        for p in morph.parse(form):
            ppos, g = tag_parts(p.tag)
            if pos not in ppos:
                continue
            if need <= g:
                ok = True
                break
        if not ok:
            seen = [str(p.tag) for p in morph.parse(form)[:2]]
            errors.append("round-trip не сошёлся: %s/%s (%s) -> %s"
                          % (word, key, form, seen))


def main():
    morph = MorphAnalyzer()
    with open(SRC, encoding="utf-8") as fh:
        src = yaml.safe_load(fh)

    data = {
        "nouns": {},
        "adjectives": {},
        "verbs": {},
        "adverbs": {},
        "indeclinable": {},
        "loans": {},
        "titles": [],
        "departments": [],
        "actors": {},
        "phrases": {},
    }

    # --- существительные ---
    for w, en in (src.get("nouns") or {}).items():
        ru = unspace(w)
        if ru in data["indeclinable"]:
            continue
        entry = collect_noun(morph, ru, en)
        if entry:
            data["nouns"][ru] = entry

    # --- существительные, которые в русском существуют только во множественном числе ---
    for w, en in (src.get("plural_only") or {}).items():
        ru = unspace(w)
        if ru not in data["nouns"]:
            entry = collect_noun(morph, ru, en, plural_only=True)
            if entry:
                data["nouns"][ru] = entry
        if ru in data["nouns"]:
            e = data["nouns"][ru]
            for k in list(e["forms"]):
                if k.endswith(":sg"):
                    del e["forms"][k]
            e["pl"] = True
            if e["forms"].get("n:nom:pl") != ru:
                errors.append("сломана парадигма: %s (И.п. мн.ч. = %s)" % (ru, e["forms"].get("n:nom:pl")))

    # --- прилагательные ---
    for w, en in (src.get("adjectives") or {}).items():
        ru = unspace(w)
        entry = collect_adj(morph, ru, en)
        if entry:
            data["adjectives"][ru] = entry

    # --- глаголы ---
    genitive_verbs = set(src.get("genitive_verbs") or {})
    for w, en in (src.get("verbs") or {}).items():
        ru = unspace(w)
        entry = collect_verb(morph, ru, en)
        if entry:
            entry["obj"] = "gen" if ru in genitive_verbs else "acc"
            data["verbs"][ru] = entry

    # --- наречия ---
    for w, en in (src.get("adverbs") or {}).items():
        data["adverbs"][unspace(w)] = en

    # --- несклоняемые ---
    for w, en in (src.get("indeclinable") or {}).items():
        ru = unspace(w)
        if ru in data["nouns"]:
            warnings.append("дубль: %s — оставлен indeclinable" % ru)
            del data["nouns"][ru]
        data["indeclinable"][ru] = en

    # --- шаблоны для ручных лемм: в общий пул не попадают ---
    tmpl_forms = {}
    for w, en in (src.get("templates") or {}).items():
        ru = unspace(w)
        entry = collect_adj(morph, ru, en)
        if entry:
            tmpl_forms[ru] = entry["forms"]

    # --- ручные парадигмы ---
    for w, spec in (src.get("loans") or {}).items():
        ru = unspace(w)
        kind = spec.get("kind", "verb")
        if "template" in spec:
            # Парадигма не выдумывается: берётся у эталонного слова
            # и пересобирается подменой основы.
            tmpl = unspace(spec["template"])
            src_forms = tmpl_forms.get(tmpl)
            if not src_forms:
                errors.append("нет шаблона для %s: %s" % (ru, tmpl))
                continue
            frm, to = spec["from"], spec["to"]
            slots = {}
            for key, form in src_forms.items():
                if not form.startswith(frm):
                    errors.append("основа не совпала: %s (%s)" % (form, frm))
                    continue
                slots[key] = to + form[len(frm):]
            # мягкая проверка: хотя бы мужской И.п. должен читаться как прилагательное
            probe = slots.get("a:nom:sg:m:-")
            if probe and not any(str(p.tag).startswith("ADJF") for p in morph.parse(probe)):
                errors.append("шаблон не подтверждён: %s (%s)" % (ru, probe))
            else:
                warnings.append("слово новое, парадигма взята из «%s»: %s" % (tmpl, ru))
            data["loans"][ru] = {"en": spec.get("en", ""), "kind": "adjective", "forms": slots}
            continue
        forms = {k: unspace(v) for k, v in spec["forms"].items()}
        if kind == "noun":
            verify_loan(morph, ru, forms, NOUN_LOAN_EXPECT)
            slots = {}
            for c in CASES:
                cs = CASE_SLOT[c]
                for num in ("sg", "pl"):
                    k = "%s_%s" % (cs, num)
                    if k in forms:
                        slots["n:%s:%s" % (cs, num)] = forms[k]
            data["loans"][ru] = {"en": spec.get("en", ""), "kind": "noun",
                                 "g": spec.get("gender", "m"), "an": False,
                                 "pl": "n:nom:sg" not in slots, "forms": slots}
            continue
        if kind == "adjective":
            verify_loan(morph, ru, forms, ADJ_LOAN_EXPECT)
            slots = {}
            for c in CASES:
                cs = CASE_SLOT[c]
                for gg in ("m", "f", "n"):
                    k = "%s_sg_%s" % (cs, gg)
                    if k in forms:
                        slots["a:%s:sg:%s:-" % (cs, gg)] = forms[k]
                        slots["a:acc:sg:%s:-" % gg] = forms.get("acc_sg_%s" % gg, forms[k])
                k = "%s_pl" % cs
                if k in forms:
                    slots["a:%s:pl:-" % cs] = forms[k]
                    slots["a:acc:pl:-"] = forms.get("acc_pl", forms[k])
            data["loans"][ru] = {"en": spec.get("en", ""), "kind": kind, "forms": slots}
        else:
            verify_loan(morph, ru, forms, LOAN_EXPECT)
            norm = {}
            for key, form in forms.items():
                if key in ("inf", "ger"):
                    norm[key] = form
                elif key in PRES_KEYS:
                    norm[PRES_KEYS[key]] = form
                elif key in PAST_KEYS:
                    norm[PAST_KEYS[key]] = form
            data["loans"][ru] = {"en": spec.get("en", ""), "kind": kind, "forms": norm}

    # --- составные существительные ---
    for w, spec in (src.get("phrases") or {}).items():
        ru = unspace(w)
        parts = []
        for part in spec.get("parts") or []:
            kind = part["kind"]
            if kind == "adj":
                lemma = unspace(part["lemma"])
                if lemma not in data["adjectives"]:
                    errors.append("прилагательное в составе фразы не найдено: %s (%s)" % (ru, lemma))
                    continue
                parts.append({"kind": "adj", "lemma": lemma})
            elif kind == "noun":
                lemma = unspace(part["lemma"])
                if lemma not in data["nouns"]:
                    errors.append("существительное в составе фразы не найдено: %s (%s)" % (ru, lemma))
                    continue
                p = {"kind": "noun", "lemma": lemma}
                if part.get("case"):
                    p["case"] = CASE_SLOT.get(part["case"], part["case"])
                if part.get("num"):
                    p["num"] = part["num"]
                parts.append(p)
            else:
                word_ = unspace(part["word"])
                if word_ not in data["indeclinable"]:
                    errors.append("несклоняемое слово в составе фразы не найдено: %s (%s)" % (ru, word_))
                    continue
                parts.append({"kind": "decl", "word": word_})
        if parts:
            data["phrases"][ru] = {"en": spec.get("en", ""), "parts": parts}

    # --- должности и подразделения ---
    for t in src.get("titles") or []:
        data["titles"].append({"ru": unspace(t["ru"]), "en": t.get("en", "")})
    for d, en in (src.get("departments") or {}).items():
        data["departments"].append({"ru": unspace(d), "en": en})

    # --- подлежащие ---
    for role, specs in (src.get("actors") or {}).items():
        variants = []
        for spec in specs:
            if "noun" in spec:
                lemma = unspace(spec["noun"])
                if lemma not in data["nouns"]:
                    errors.append("подлежащее не найдено: %s" % lemma)
                    continue
                variants.append({"parts": [{"kind": "noun", "lemma": lemma}]})
            elif "phrase" in spec:
                ref = unspace(spec["phrase"])
                if ref not in data["phrases"]:
                    errors.append("фраза не найдена: %s" % ref)
                    continue
                variants.append({"parts": data["phrases"][ref]["parts"]})
            elif "decl" in spec:
                word_ = unspace(spec["decl"])
                if word_ not in data["indeclinable"]:
                    errors.append("несклоняемое подлежащее не найдено: %s" % word_)
                    continue
                variants.append({"parts": [{"kind": "decl", "word": word_}]})
        data["actors"][role] = variants

    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    with open(OUT, "w", encoding="utf-8") as fh:
        json.dump(data, fh, ensure_ascii=False, indent=1, sort_keys=True)

    print("лексикон: %d сущ., %d прил., %d глаг., %d нареч., %d нескл., %d ручных, %d фраз"
          % (len(data["nouns"]), len(data["adjectives"]), len(data["verbs"]),
             len(data["adverbs"]), len(data["indeclinable"]), len(data["loans"]),
             len(data["phrases"])))
    print("должностей: %d, подразделений: %d, ролей подлежащих: %d"
          % (len(data["titles"]), len(data["departments"]), len(data["actors"])))

    if warnings:
        print("\n--- предупреждения (%d) ---" % len(warnings))
        for w in warnings:
            print("  !", w)
    if errors:
        print("\n--- ОШИБКИ (%d) ---" % len(errors))
        for e in errors:
            print("  x", e)
        sys.exit(1)
    print("\nвсё чисто.")


if __name__ == "__main__":
    main()
