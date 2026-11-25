// Package morph превращает лемму в нужную форму: склоняет существительное,
// согласует прилагательное, спрягает глагол.
//
// Ключи форм совпадают с теми, что собирает tools/build_lexicon.py:
//
//	существительное: n:<падеж>:<число>
//	прилагательное:  a:<падеж>:<число>:<род>:<одушевлённость>
//	глагол:          pres:<число>:<лицо>, past:<число>:<род>
package morph

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/thegalkin/corporate-slop-generator-russian/internal/lexicon"
)

// Падежи.
const (
	CaseNom  = "nom"
	CaseGen  = "gen"
	CaseDat  = "dat"
	CaseAcc  = "acc"
	CaseIns  = "ins"
	CasePrep = "prep"
)

// Число.
const (
	NumSg = "sg"
	NumPl = "pl"
)

// Род.
const (
	GenM = "m"
	GenF = "f"
	GenN = "n"
)

// Ctx — контекст генерации: лексикон и источник случайности.
type Ctx struct {
	L *lexicon.Lexicon
	R *rand.Rand
	// Pools — словари для наполнения слотов.
	Pools *Pools

	// Subject — последнее отрисованное подлежащее; по нему согласуются
	// глагол и прилагательные.
	Subject *NounInfo
}

// Pools — списки лемм, из которых набираются слоты.
type Pools struct {
	Nouns   []string
	Adjs    []string
	Verbs   []string
	Advs    []string
	Decls   []string
	Phrases []string
}

// BuildPools собирает пулы один раз на весь лексикон.
func BuildPools(l *lexicon.Lexicon) *Pools {
	p := &Pools{}
	for lemma := range l.Nouns {
		p.Nouns = append(p.Nouns, lemma)
	}
	for lemma := range l.Adjectives {
		p.Adjs = append(p.Adjs, lemma)
	}
	for lemma := range l.Verbs {
		p.Verbs = append(p.Verbs, lemma)
	}
	for lemma := range l.Adverbs {
		p.Advs = append(p.Advs, lemma)
	}
	for lemma := range l.Indeclinable {
		p.Decls = append(p.Decls, lemma)
	}
	for key := range l.Phrases {
		p.Phrases = append(p.Phrases, key)
	}
	return p
}

// NounInfo — всё, что нужно, чтобы согласовать с существительным.
type NounInfo struct {
	Form   string
	Gender string
	Anim   bool
	Case   string
	Number string
}

// NounKey собирает ключ формы существительного.
func NounKey(cs, num string) string { return "n:" + cs + ":" + num }

// animFor: одушевлённость важна только в винительном падеже.
func animFor(cs string, anim bool) string {
	if cs != CaseAcc {
		return "-"
	}
	if anim {
		return "anim"
	}
	return "inan"
}

// NounForm отдаёт форму существительного.
func (c *Ctx) NounForm(lemma, cs, num string) (string, bool) {
	n, ok := c.L.Noun(lemma)
	if !ok {
		return "", false
	}
	f, ok := n.Forms[NounKey(cs, num)]
	return f, ok
}

// AdjForm отдаёт форму прилагательного, согласованную с указанным существительным.
//
// Во множественном числе русский язык снимает род, поэтому ключ короче:
//
//	ед. число:  a:<падеж>:sg:<род>:-
//	мн. число:  a:<падеж>:pl:-
func (c *Ctx) AdjForm(lemma string, info *NounInfo) (string, bool) {
	if info == nil {
		return "", false
	}
	a, ok := c.L.Adj(lemma)
	if !ok {
		return "", false
	}
	if info.Number == NumPl {
		if info.Case == CaseAcc {
			if f, ok := a.Forms[fmt.Sprintf("a:acc:pl:%s", animFor(CaseAcc, info.Anim))]; ok {
				return f, true
			}
			if f, ok := a.Forms["a:acc:pl:-"]; ok {
				return f, true
			}
		}
		if f, ok := a.Forms[fmt.Sprintf("a:%s:pl:-", info.Case)]; ok {
			return f, true
		}
		return "", false
	}
	gender := info.Gender
	if info.Case == CaseAcc {
		if f, ok := a.Forms[fmt.Sprintf("a:acc:sg:%s:%s", gender, animFor(CaseAcc, info.Anim))]; ok {
			return f, true
		}
		if f, ok := a.Forms[fmt.Sprintf("a:acc:sg:%s:-", gender)]; ok {
			return f, true
		}
	}
	if f, ok := a.Forms[fmt.Sprintf("a:%s:sg:%s:-", info.Case, gender)]; ok {
		return f, true
	}
	return "", false
}

// VerbForm отдаёт форму глагола.
//
// person: 1, 2 или 3. tense: pres или past.
// Если у глагола нет запрошенного вида, берётся другой.
func (c *Ctx) VerbForm(lemma, tense, num, gender string, person int) (string, bool) {
	v, ok := c.L.Verb(lemma)
	if !ok {
		return "", false
	}
	key := ""
	switch tense {
	case "inf":
		key = "inf"
	case "ger":
		key = "ger"
	case "past":
		g := gender
		if g == "" {
			g = GenM
		}
		if num == NumPl {
			g = "pl"
		}
		key = fmt.Sprintf("past:%s:%s", num, g)
	default:
		key = fmt.Sprintf("pres:%s:%d", num, person)
	}
	order := []string{"impf", "pf"}
	if v.Default == "pf" {
		order = []string{"pf", "impf"}
	}
	for _, aspect := range order {
		if forms, ok := v.Aspects[aspect]; ok {
			if f, ok := forms[key]; ok {
				return f, true
			}
		}
	}
	return "", false
}

// RenderParts отрисовывает составной кусок: прилагательные согласуются
// с ближайшим следующим существительным, у которого задан свой падеж.
//
// Возвращает текст и «головное» существительное — по нему согласуются
// внешние прилагательные и сказуемые.
func (c *Ctx) RenderParts(parts []lexicon.Part, cs, num string) (string, *NounInfo) {
	// 1. Где стоят существительные: прилагательному нужно знать, с каким
	//    словом согласовываться, ещё до отрисовки.
	var nounIdx []int
	for i, p := range parts {
		if p.Kind == "noun" {
			nounIdx = append(nounIdx, i)
		}
	}

	// 2. Существительные.
	forms := make([]string, len(parts))
	infos := make([]*NounInfo, len(parts))
	for _, i := range nounIdx {
		p := parts[i]
		case_ := p.Case
		if case_ == "" {
			case_ = cs
		}
		num_ := num
		if p.Num != "" {
			num_ = p.Num
		}
		form, ok := c.NounForm(p.Lemma, case_, num_)
		if !ok {
			// слово, бывающее только во множественном числе
			if n0, ok0 := c.L.Noun(p.Lemma); ok0 && n0.Pl {
				num_ = NumPl
				form, ok = c.NounForm(p.Lemma, case_, num_)
			}
		}
		if !ok {
			form, _ = c.NounForm(p.Lemma, CaseNom, num_)
		}
		n, _ := c.L.Noun(p.Lemma)
		forms[i] = form
		infos[i] = &NounInfo{Form: form, Gender: n.G, Anim: n.An, Case: case_, Number: num_}
	}

	// 3. Прилагательные согласуются с ближайшим следующим существительным.
	for i, p := range parts {
		if p.Kind != "adj" {
			continue
		}
		next := -1
		for _, j := range nounIdx {
			if j > i {
				next = j
				break
			}
		}
		if next < 0 || infos[next] == nil {
			continue
		}
		if f, ok := c.AdjForm(p.Lemma, infos[next]); ok {
			forms[i] = f
		}
	}

	// 4. Несклоняемые слова.
	for i, p := range parts {
		if p.Kind == "decl" {
			forms[i] = p.Word
		}
	}

	// 5. Сборка текста.
	pieces := make([]string, 0, len(parts))
	for _, f := range forms {
		if f != "" {
			pieces = append(pieces, f)
		}
	}

	// Головное существительное — последнее в цепочке.
	var head *NounInfo
	for _, i := range nounIdx {
		if infos[i] != nil {
			head = infos[i]
		}
	}
	return strings.Join(pieces, " "), head
}
