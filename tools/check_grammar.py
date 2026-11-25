#!/usr/bin/env python3
"""
Независимая проверка грамматики вывода генератора.

Генерирует текст бинарём cbsg, разбирает его анализатором pymorphy3 и ищет
нарушения согласования:

  * прилагательное не согласовано с существительным (род / число / падеж);
  * глагол в 3-м лице не согласован с подлежащим по числу;
  * слова, которых нет в словаре.

Проверка неоднозначна: у русского слова часто несколько разборов
(«ценности» — это и род. ед. ж. р. от «ценность», и им. мн. ч. от «ценность»).
Поэтому слово описывается множеством разборов, а нарушение — это когда
множества несовместимы.

Запуск:
    .venv/bin/python tools/check_grammar.py [-n 3000] [--seed 1]
"""

import argparse
import collections
import json
import os
import re
import subprocess
import sys

from pymorphy3 import MorphAnalyzer

HERE = os.path.dirname(os.path.abspath(__file__))
LEX_PATH = os.path.join(os.path.dirname(HERE), "internal", "lexicon", "data", "lexicon.json")

TOKEN = re.compile(r"[А-Яа-яЁёA-Za-z-]+")
CLAUSE = re.compile(r"[^;,.!?]+")
CASES = ("nomn", "gent", "datv", "accs", "ablt", "loct")
GENDERS = ("masc", "femn", "neut")

# служебные слова, которые анализатор разбирает как прилагательные
FUNCTION_WORDS = {
    "тем", "тот", "то", "такое", "такой", "такие", "сам", "само", "сами",
    "всё", "все", "всю", "который", "которая", "которые", "чего", "чём",
}


def tag_parts(t):
    s = str(t)
    head, _, rest = s.partition(" ")
    parts = head.split(",")
    gram = set(parts[1:])
    if rest:
        gram |= {g for g in rest.split(",") if g}
    return parts[0], gram


class Word:
    """Слово со всеми возможными разборами."""

    def __init__(self, text, parses, adverbs, indecl, known_nouns=()):
        self.text = text
        low = text.lower()
        self.known = bool(parses) and any(p.is_known for p in parses)
        self.forms = set()      # (род, число, падеж)
        self.numbers = set()
        self.third_person = False
        self.pos = set()
        self.skip = low in adverbs or low in indecl or low in FUNCTION_WORDS
        self.finite_verb = False
        self.known_noun = low in known_nouns
        for p in parses:
            pos, gram = tag_parts(p.tag)
            self.pos.add(pos)
            n = "plur" if "plur" in gram else "sing"
            g = next((x for x in GENDERS if x in gram), None)
            if n == "plur" and g is None:
                g = "-"  # во множественном числе род снимается
            c = next((x for x in CASES if x in gram), None)
            if pos.startswith("ADJF") or pos.startswith("PRTF") or pos.startswith("NOUN"):
                if g and n and c:
                    self.forms.add((g, n, c))
                self.numbers.add(n)
            elif pos == "VERB":
                self.numbers.add(n)
                self.third_person = "3per" in gram
                # «бенчмарким» — это и причастие, и глагол; в позиции сказуемого
                # это глагол, а не прилагательное
                if any(p in gram for p in ("1per", "2per", "3per")):
                    self.finite_verb = True

    @property
    def is_adjective(self):
        if self.finite_verb:
            return False
        if "ADJF" not in self.pos and "PRTF" not in self.pos:
            return False
        # слово из нашего лексикона мы знаем точно: «данных» — это
        # родительный падеж от «данные», а не прилагательное «данный»
        if self.known_noun:
            return False
        # у прилагательного во множественном числе род снимается, поэтому
        # в формах множественного числа рода быть не должно. Если род есть —
        # это не прилагательное, а существительное в родительном падеже.
        for g, n, _ in self.forms:
            if n == "plur" and g != "-":
                return False
        return True


def compatible(a, b):
    """Совместимы ли формы прилагательного a и существительного b."""
    for ga, na, ca in a:
        for gb, nb, cb in b:
            if na != nb or ca != cb:
                continue
            if ga != "-" and gb != "-" and ga != gb:
                continue
            return True
    return False


def check_adjective_agreement(words):
    """Прилагательные (подряд) согласуются с существительным за ними."""
    problems = []
    i = 0
    while i < len(words):
        if words[i].skip or not words[i].is_adjective:
            i += 1
            continue
        run = []
        j = i
        while j < len(words) and not words[j].skip and words[j].is_adjective:
            run.append(words[j])
            j += 1
        if j < len(words) and "NOUN" in words[j].pos and not words[j].skip:
            noun = words[j]
            # прилагательное и существительное — одна пара, только если
            # у них совпадает падеж; иначе это просто соседние слова
            adj_cases = {c for (_, _, c) in run[0].forms}
            noun_cases = {c for (_, _, c) in noun.forms}
            if adj_cases & noun_cases:
                for adj in run:
                    if adj.forms and noun.forms and not compatible(adj.forms, noun.forms):
                        problems.append("прилагательное %s не согласовано с %s (%s / %s)"
                                        % (adj.text, noun.text,
                                           sorted(adj.forms), sorted(noun.forms)))
            i = j
            continue
        i = j if j > i else i + 1
    return problems


def check_verb_agreement(words):
    """Глагол в 3-м лице согласуется с подлежащим по числу.

    Проверяем внутри одной части предложения: через запятую подлежащее
    может смениться.
    """
    problems = []
    subject = None
    for w in words:
        if w.skip:
            continue
        if "NOUN" in w.pos and any(c == "nomn" for (_, _, c) in w.forms):
            subject = w
        if "VERB" in w.pos and w.third_person and subject is not None:
            if not (w.numbers & subject.numbers):
                problems.append("глагол %s (%s) не согласован с подлежащим %s (%s)"
                                % (w.text, sorted(w.numbers), subject.text, sorted(subject.numbers)))
            subject = None
    return problems


def is_expected(token, loans, indecl):
    """Займы и транслитерации словаря не знают — это ожидаемо."""
    if token in indecl:
        return True
    for lemma in loans:
        if token.startswith(lemma) or lemma.startswith(token):
            return True
    return False


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-n", type=int, default=1000, help="сколько предложений проверить")
    ap.add_argument("--seed", type=int, default=1)
    ap.add_argument("--show", type=int, default=10, help="сколько примеров показать")
    args = ap.parse_args()

    morph = MorphAnalyzer()

    lex = {}
    try:
        with open(LEX_PATH, encoding="utf-8") as fh:
            lex = json.load(fh)
    except OSError:
        pass
    adverbs = {k.lower() for k in lex.get("adverbs", {})}
    indecl = {k.lower() for k in lex.get("indeclinable", {})}
    loans = {k.lower() for k in lex.get("loans", {})}
    indecl |= {"фраза"}

    # слова, которые есть в нашем лексиконе, известны нам точно:
    # «данных» — это родительный падеж от «данные», а не прилагательное «данный»
    known_nouns = set()
    for entry in lex.get("nouns", {}).values():
        known_nouns |= {f.lower() for f in entry.get("forms", {}).values()}
    for entry in lex.get("loans", {}).values():
        if entry.get("kind") == "noun":
            known_nouns |= {f.lower() for f in entry.get("forms", {}).values()}

    out = subprocess.run(
        ["go", "run", "./cmd/cbsg", "-n", str(args.n), "-seed", str(args.seed), "-quiet"],
        capture_output=True, text=True, check=True,
    ).stdout

    # многословные несклоняемые («совет директоров») и наречия
    # («на опережение») — это одна конструкция, а не несколько слов
    phrases = [p for p in indecl if " " in p]
    phrases += [a.replace("_", " ") for a in adverbs if " " in a or "_" in a]
    for p in sorted(phrases, key=len, reverse=True):
        out = re.sub(re.escape(p), " ФРАЗА ", out, flags=re.IGNORECASE)
    out = re.sub(r"\bФРАЗА\b", "ФРАЗА", out)

    sentences = [s.strip() for s in re.split(r"(?<=[.])\s+", out) if s.strip()]
    kinds = collections.Counter()
    examples = collections.defaultdict(list)
    unknown = collections.Counter()
    expected_unknown = collections.Counter()

    for sentence in sentences:
        clause_texts = [c.strip() for c in CLAUSE.findall(sentence)]
        clause_words = []
        for ct in clause_texts:
            ws = [Word(t, morph.parse(t), adverbs, indecl, known_nouns) for t in TOKEN.findall(ct)]
            clause_words.append(ws)
            for w in ws:
                if not w.known:
                    if is_expected(w.text.lower(), loans, indecl):
                        expected_unknown[w.text.lower()] += 1
                    else:
                        unknown[w.text.lower()] += 1
            for problem in check_adjective_agreement(ws):
                kinds["согласование прилагательного"] += 1
                examples["согласование прилагательного"].append((sentence, problem))
            for problem in check_verb_agreement(ws):
                kinds["согласование глагола"] += 1
                examples["согласование глагола"].append((sentence, problem))

    print("проверено предложений: %d" % len(sentences))
    if not kinds:
        print("нарушений не найдено.")
    for kind, count in kinds.most_common():
        print("\n%s: %d" % (kind, count))
        for sentence, problem in examples[kind][:args.show]:
            print("   %s\n      -> %s" % (sentence.strip(), problem))

    if unknown:
        print("\nслов вне словаря: %d уникальных" % len(unknown))
        for w, c in unknown.most_common(20):
            print("   %-26s %d" % (w, c))
    if expected_unknown:
        print("\nожидаемые займы (нет в OpenCorpora): %d уникальных" % len(expected_unknown))
        for w, c in expected_unknown.most_common(10):
            print("   %-26s %d" % (w, c))

    return 1 if kinds else 0


if __name__ == "__main__":
    sys.exit(main())
