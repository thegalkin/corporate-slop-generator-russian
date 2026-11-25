// Package lexicon хранит морфологические данные русского корпоративного
// жаргона и умеет отдавать нужную форму слова.
//
// Данные собираются офлайн скриптом tools/build_lexicon.py из OpenCorpora:
// для каждой леммы вытаскивается вся парадигма, и формы раскладываются по
// ключам вида "n:gen:sg" (существительное) или "a:acc:pl:-" (прилагательное).
// В рантайме это просто поиск по таблице — ни OpenCorpora, ни Python не нужны.
package lexicon

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed data/lexicon.json
var raw []byte

// Noun — существительное с родом, одушевлённостью и полной парадигмой.
type Noun struct {
	En    string            `json:"en"`
	G     string            `json:"g"`
	An    bool              `json:"an"`
	Pl    bool              `json:"pl"`
	Forms map[string]string `json:"forms"`
}

// Adj — прилагательное; в единственном числе три рода, во множественном
// род снимается, поэтому ключ "a:nom:pl:-" без рода.
type Adj struct {
	En    string            `json:"en"`
	Forms map[string]string `json:"forms"`
}

// Verb — глагол; формы разложены по видам (совершенный/несовершенный).
// Obj — падеж дополнения: у большинства глаголов винительный,
// но некоторые требуют родительного («достигать цели», «требовать усилий»).
type Verb struct {
	En      string                       `json:"en"`
	Default string                       `json:"default"`
	Obj     string                       `json:"obj"`
	Aspects map[string]map[string]string `json:"aspects"`
}

// Part — кусок составного существительного.
type Part struct {
	Kind  string `json:"kind"` // noun | adj | decl
	Lemma string `json:"lemma"`
	Word  string `json:"word"`
	Case  string `json:"case"` // если задан, падеж не берётся у слота
	Num   string `json:"num"`  // если задан, число не берётся у слота
}

// Phrase — составное существительное.
type Phrase struct {
	En    string `json:"en"`
	Parts []Part `json:"parts"`
}

// Actor — вариант подлежащего.
type Actor struct {
	Parts []Part `json:"parts"`
}

// Title — должность.
type Title struct {
	Ru string `json:"ru"`
	En string `json:"en"`
}

// Department — подразделение (всегда в родительном падеже).
type Department struct {
	Ru string `json:"ru"`
	En string `json:"en"`
}

// Loan — лемма, которой нет в словаре: парадигма задана вручную
// и проверена round-trip'ом на этапе сборки.
type Loan struct {
	En    string            `json:"en"`
	Kind  string            `json:"kind"` // noun | adjective | verb
	G     string            `json:"g"`
	An    bool              `json:"an"`
	Pl    bool              `json:"pl"`
	Forms map[string]string `json:"forms"`
}

// Lexicon — весь лексикон разом.
type Lexicon struct {
	Nouns        map[string]Noun    `json:"nouns"`
	Adjectives   map[string]Adj     `json:"adjectives"`
	Verbs        map[string]Verb    `json:"verbs"`
	Adverbs      map[string]string  `json:"adverbs"`
	Indeclinable map[string]string  `json:"indeclinable"`
	Loans        map[string]Loan    `json:"loans"`
	Titles       []Title            `json:"titles"`
	Departments  []Department       `json:"departments"`
	Actors       map[string][]Actor `json:"actors"`
	Phrases      map[string]Phrase  `json:"phrases"`
}

var cached *Lexicon

// Load читает встроенный лексикон и подмешивает в общие таблицы леммы,
// у которых парадигма задана вручную.
func Load() (*Lexicon, error) {
	if cached != nil {
		return cached, nil
	}
	l := &Lexicon{}
	if err := json.Unmarshal(raw, l); err != nil {
		return nil, fmt.Errorf("лексикон: %w", err)
	}
	for lemma, loan := range l.Loans {
		switch loan.Kind {
		case "noun":
			l.Nouns[lemma] = Noun{En: loan.En, G: loan.G, An: loan.An, Pl: loan.Pl, Forms: loan.Forms}
		case "adjective":
			l.Adjectives[lemma] = Adj{En: loan.En, Forms: loan.Forms}
		case "verb":
			l.Verbs[lemma] = Verb{En: loan.En, Default: "pf", Aspects: map[string]map[string]string{"pf": loan.Forms}}
		}
	}
	cached = l
	return l, nil
}

// Noun возвращает существительное по лемме.
func (l *Lexicon) Noun(lemma string) (Noun, bool) {
	n, ok := l.Nouns[lemma]
	return n, ok
}

// Adj возвращает прилагательное по лемме.
func (l *Lexicon) Adj(lemma string) (Adj, bool) {
	a, ok := l.Adjectives[lemma]
	return a, ok
}

// Verb возвращает глагол по лемме.
func (l *Lexicon) Verb(lemma string) (Verb, bool) {
	v, ok := l.Verbs[lemma]
	return v, ok
}

// ObjCase возвращает падеж, который требует глагол.
func (l *Lexicon) ObjCase(lemma string) string {
	if v, ok := l.Verbs[lemma]; ok && v.Obj != "" {
		return v.Obj
	}
	return "acc"
}
