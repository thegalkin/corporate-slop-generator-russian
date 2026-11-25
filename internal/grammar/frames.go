package grammar

import (
	"github.com/thegalkin/corporate-slop-generator-russian/internal/morph"
)

// Хелперы для сборки предложений.

func randomNoun(c *morph.Ctx) string {
	p := c.Pools.Nouns
	return p[c.R.Intn(len(p))]
}

func randomAdj(c *morph.Ctx) string {
	p := c.Pools.Adjs
	return p[c.R.Intn(len(p))]
}

func randomVerb(c *morph.Ctx) string {
	p := c.Pools.Verbs
	return p[c.R.Intn(len(p))]
}

func advPool(c *morph.Ctx) []string { return c.Pools.Advs }

func declPool(c *morph.Ctx) []string { return c.Pools.Decls }

// resolveActor выбирает подлежащее: вариант из пула роли, число и текст.
func resolveActor(c *morph.Ctx, role, numSpec string) (string, *morph.NounInfo, string) {
	variants := c.L.Actors[role]
	if len(variants) == 0 {
		return "", nil, ""
	}
	act := variants[c.R.Intn(len(variants))]
	num := numSpec
	if num == "" || num == "auto" {
		num = morph.NumSg
		if c.R.Intn(100) < 30 {
			num = morph.NumPl
		}
	}
	// фраза, оканчивающаяся несклоняемым словом, существует только в ед. числе
	if n := len(act.Parts); n > 0 && act.Parts[n-1].Kind == "decl" {
		num = morph.NumSg
	}
	text, info := c.RenderParts(act.Parts, morph.CaseNom, num)
	return text, info, num
}

// --- сокращения для сборки рамок ---

func subj(role string) Part { return &Subj{Role: role, Num: "auto"} }

func subjAdj(role string) Part { return &SubjAdj{Role: role} }

func noun(cs string) Part { return &NounSlot{Case: cs, Number: "auto"} }

func attr(cs string) Part { return &AttrSlot{Case: cs, Number: "auto"} }

func decl(cs string) Part { return &DeclSlot{Case: cs} }

func adv() Part { return &AdvSlot{} }

func inf() Part { return &InfSlot{} }

func title() Part { return &TitleSlot{} }

// vp — сказуемое в 3-м лице, число по подлежащему.
func vp() Part { return &VP{Person: 3, Number: "auto", Tense: "pres"} }

// vpWe — сказуемое в 1-м лице множественного числа («мы»).
func vpWe() Part { return &VP{Person: 1, Number: "pl", Tense: "pres"} }

// vpPast — сказуемое в прошедшем времени; род берётся у подлежащего,
// поэтому «команда зафиксировала», а «комитет зафиксировал».
func vpPast() Part { return &VP{Person: 3, Number: "auto", Tense: "past"} }

func phrase(key string) Part { return &PhraseSlot{Key: key, Case: morph.CaseNom} }

func phraseGen(key string) Part { return &PhraseSlot{Key: key, Case: morph.CaseGen} }

// Frames — предложения. Каждая рамка сама решает, в каком падеже стоит
// каждое существительное: падеж — свойство рамки, а не слова.
var Frames = []Frame{
	// подлежащее, сказуемое, дополнение
	{subj("team"), adv(), vp()},
	{subj("committee"), vp()},
	{subj("market"), adv(), vp()},
	{subjAdj("committee"), vp()},
	{subjAdj("team"), adv(), vp()},
	{subj("office"), vp(), noun(morph.CaseGen)},

	// предлоги, требующие своего падежа
	{Lit("в рамках"), noun(morph.CaseGen), subj("team"), vp()},
	{Lit("в рамках"), attr(morph.CaseGen), subj("committee"), vp()},
	{Lit("с точки зрения"), noun(morph.CaseGen), subj("team"), vp()},
	{Lit("в части"), noun(morph.CaseGen), subj("committee"), vp()},
	{Lit("в контексте"), noun(morph.CaseGen), subj("market"), vp()},
	{Lit("на уровне"), noun(morph.CaseGen), subj("team"), vp()},
	{Lit("за счёт"), noun(morph.CaseGen), subj("committee"), vp()},
	{Lit("потребность"), &PrepSlot{Prep: "в"}, subj("team"), vp()},
	{Lit("переход к"), noun(morph.CaseDat), subj("committee"), vp()},
	{Lit("согласно"), noun(morph.CaseDat), Lit(","), subj("team"), vp()},
	{Lit("в формате"), noun(morph.CaseGen), subj("office"), vp()},
	{Lit("не обходится без"), noun(morph.CaseGen)},
	{Lit("речь идёт не только"), &PrepSlot{Prep: "о"}, Lit(", но и"), &PrepSlot{Prep: "о"}},

	// «мы»
	{Lit("мы"), vpWe()},
	{Lit("мы"), vpWe(), Lit(", а"), subj("team"), vp()},

	// конструкции с обстоятельными
	{Lit("команда"), noun(morph.CaseGen), Lit("не может не"), inf(), noun(morph.CaseAcc)},
	{Lit("отсутствие"), noun(morph.CaseGen), Lit("не является"), noun(morph.CaseIns), Lit("для"), noun(morph.CaseGen)},
	{Lit("уровень"), noun(morph.CaseGen), Lit("определяет"), noun(morph.CaseAcc)},
	{Lit("инициатива"), noun(morph.CaseGen), Lit("нацелена на"), noun(morph.CaseAcc)},
	{Lit("в итоге"), adv(), subj("committee"), vp()},
	{Lit("хайп вокруг"), noun(morph.CaseGen), Lit("не заменяет"), noun(morph.CaseAcc)},
	{Lit("коммитмент по"), noun(morph.CaseDat), Lit("требует"), noun(morph.CaseGen)},

	// зачинки
	{Lit("таким образом,"), subj("team"), vp()},
	{Lit("в то же время"), subj("committee"), adv(), vp()},
	{Lit("тем не менее,"), subj("market"), vp()},
	{Lit("вместе с тем,"), subj("team"), vp()},

	// должности
	{title(), vp()},
	{title(), adv(), vp()},

	// устойчивые обороты корпоративного русского
	{phrase("цифровая трансформация"), vp()},
	{phrase("клиентоориентированный подход"), vp()},
	{phrase("операционная эффективность"), vp()},
	{phrase("стратегическая сессия"), vp()},
	{phrase("деловая культура"), vp()},
	{phrase("корпоративная стратегия"), vp()},
	{phrase("ценностное предложение"), vp()},
	{phrase("модель бизнеса"), vp()},
	{phrase("клиентская база"), vp()},
	{phrase("человеческий капитал"), vp()},
	{phrase("интеллектуальный капитал"), vp()},

	// транслитерации: их не склоняют, но вполне можно упомянуть
	{subj("team"), &VP{Person: 3, Number: "auto", Tense: "pres", Decl: true}},
	{subj("committee"), &VP{Person: 3, Number: "auto", Tense: "past", Decl: true}},
	{subj("market"), &VP{Person: 3, Number: "auto", Tense: "pres", Decl: true}},
	{subj("office"), &VP{Person: 3, Number: "auto", Tense: "past", Decl: true}},
	{Lit("в контексте"), noun(morph.CasePrep), subj("team"), &VP{Person: 3, Number: "auto", Tense: "pres", Decl: true}},
	{Lit("после"), noun(morph.CaseGen), subj("committee"), &VP{Person: 3, Number: "auto", Tense: "pres", Decl: true}},
}

// Connectors — связки между частями сложного предложения.
// Только такие, после которых любая рамка даёт осмысленное предложение.
var Connectors = []string{
	", при этом,",
	"; тем не менее,",
	", в то же время,",
	", таким образом,",
	"; в конечном счёте,",
	", тогда как",
	"; в итоге,",
	", в результате",
}
