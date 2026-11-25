// Package grammar описывает предложения русского корпоративного булшита.
//
// В отличие от английского оригинала, где единственное, что гуляет по
// предложению, — число, здесь каждый слот знает, в каком падеже он стоит:
//
//	{Subj}      — подлежащее (именительный падеж)
//	{Noun gen}  — существительное в родительном падеже
//	{Verb}      — глагол, согласованный с подлежащим
//	{Attr gen}  — прилагательное с существительным, согласованы по роду и падежу
//
// Падеж задаёт рамка, а не словарь: предлог «в рамках» требует родительного,
// «к» — дательного, «на уровне» — предложного.
package grammar

import (
	"strings"

	"github.com/thegalkin/corporate-slop-generator-russian/internal/lexicon"
	"github.com/thegalkin/corporate-slop-generator-russian/internal/morph"
)

// Part — кусок предложения.
type Part interface {
	// Render отрисовывает кусок, опираясь на уже отрисованное подлежащее.
	Render(c *morph.Ctx) string
	// Prepare вызывается до отрисовки: подлежащее нужно выбрать заранее,
	// чтобы сказуемое знало про него.
	Prepare(c *morph.Ctx)
}

// Lit — литеральный текст.
type Lit string

func (l Lit) Prepare(c *morph.Ctx)       {}
func (l Lit) Render(c *morph.Ctx) string { return string(l) }

// Subj — подлежащее: «комитет», «рабочая группа», «эксперты».
//
// Num: "sg", "pl" или "auto".
type Subj struct {
	Role string
	Num  string

	text string
	info *morph.NounInfo
	num  string
}

func (s *Subj) Prepare(c *morph.Ctx) {
	s.text, s.info, s.num = resolveActor(c, s.Role, s.Num)
	if s.info == nil {
		// подлежащее без существительного: сказуемое не с чем согласовать
		s.num = morph.NumSg
	}
}

func (s *Subj) Render(c *morph.Ctx) string {
	c.Subject = s.info
	return s.text
}

// SubjAdj — прилагательное перед подлежащим: «цифровой комитет перезапускает…».
// Прилагательное согласуется с подлежащим по роду, числу и падежу.
type SubjAdj struct {
	Role string

	adj string
	sub *Subj
}

func (s *SubjAdj) Prepare(c *morph.Ctx) {
	// рамки живут в общей таблице, поэтому состояние надо сбрасывать:
	// иначе прилагательное от прошлого предложения «залипнет» на новом.
	s.adj = ""
	sub := &Subj{Role: s.Role, Num: "auto"}
	sub.Prepare(c)
	s.sub = sub
	if sub.info == nil {
		// подлежащее без существительного (например, «совет директоров»):
		// согласовывать нечего
		return
	}
	for i := 0; i < 40; i++ {
		lemma := randomAdj(c)
		if f, ok := c.AdjForm(lemma, sub.info); ok {
			s.adj = f
			return
		}
	}
}

func (s *SubjAdj) Render(c *morph.Ctx) string {
	if s.adj == "" {
		return s.sub.Render(c)
	}
	return s.adj + " " + s.sub.Render(c)
}

// NounSlot — существительное в заданном падеже.
//
// Number: "sg", "pl" или "auto".
type NounSlot struct {
	Case   string
	Number string
}

func (n *NounSlot) Prepare(c *morph.Ctx) {}

func (n *NounSlot) Render(c *morph.Ctx) string {
	// иногда — составное существительное («модель бизнеса», «поток ценности»)
	if len(c.Pools.Phrases) > 0 && c.R.Intn(100) < 20 {
		if text := renderPhrase(c, c.Pools.Phrases[c.R.Intn(len(c.Pools.Phrases))], n.Case, n.Number); text != "" {
			return text
		}
	}
	for i := 0; i < 40; i++ {
		lemma := randomNoun(c)
		noun, ok := c.L.Noun(lemma)
		if !ok {
			continue
		}
		num := pickNumber(c, n.Number, noun.Pl)
		if f, ok := c.NounForm(lemma, n.Case, num); ok {
			return f
		}
	}
	return "процессы"
}

// renderPhrase отрисовывает составное существительное из лексикона.
func renderPhrase(c *morph.Ctx, key, cs, numSpec string) string {
	ph, ok := c.L.Phrases[key]
	if !ok {
		return ""
	}
	num := numSpec
	if num == "" || num == "auto" {
		num = morph.NumSg
		if c.R.Intn(100) < 30 {
			num = morph.NumPl
		}
	}
	text, _ := c.RenderParts(ph.Parts, cs, num)
	return text
}

// pickNumber выбирает число с учётом того, что некоторые слова бывают
// только во множественном.
func pickNumber(c *morph.Ctx, spec string, pluralOnly bool) string {
	num := spec
	if num == "" || num == "auto" {
		num = morph.NumSg
		if c.R.Intn(100) < 35 {
			num = morph.NumPl
		}
	}
	if pluralOnly {
		num = morph.NumPl
	}
	return num
}

// PhraseSlot — конкретное составное существительное («цифровая трансформация»).
//
// Если фраза стоит в позиции подлежащего, её число согласуется со сказуемым,
// поэтому слот ведёт себя как подлежащее.
type PhraseSlot struct {
	Key  string
	Case string

	text string
	info *morph.NounInfo
	num  string
}

func (p *PhraseSlot) Prepare(c *morph.Ctx) {
	ph, ok := c.L.Phrases[p.Key]
	if !ok {
		p.text, p.info, p.num = "", nil, morph.NumSg
		return
	}
	num := morph.NumSg
	if c.R.Intn(100) < 30 {
		num = morph.NumPl
	}
	p.text, p.info = c.RenderParts(ph.Parts, p.Case, num)
	p.num = num
}

func (p *PhraseSlot) Render(c *morph.Ctx) string {
	if p.info != nil {
		c.Subject = p.info
	}
	return p.text
}

// DeclSlot — несклоняемое слово (митап, хайп, перформанс). Такие слова
// безопасны только в именительном и винительном падеже.
type DeclSlot struct {
	Case string
}

func (d *DeclSlot) Prepare(c *morph.Ctx) {}

func (d *DeclSlot) Render(c *morph.Ctx) string {
	words := declPool(c)
	if len(words) == 0 {
		return "митап"
	}
	return words[c.R.Intn(len(words))]
}

// AttrSlot — прилагательное с существительным: «синергетическая трансформация».
// Прилагательное подбирается под род и падеж существительного.
type AttrSlot struct {
	Case   string
	Number string
}

func (a *AttrSlot) Prepare(c *morph.Ctx) {}

func (a *AttrSlot) Render(c *morph.Ctx) string {
	for i := 0; i < 40; i++ {
		nounLemma := randomNoun(c)
		noun, ok := c.L.Noun(nounLemma)
		if !ok {
			continue
		}
		num := pickNumber(c, a.Number, noun.Pl)
		adjLemma := randomAdj(c)
		parts := []lexicon.Part{
			{Kind: "adj", Lemma: adjLemma},
			{Kind: "noun", Lemma: nounLemma},
		}
		text, _ := c.RenderParts(parts, a.Case, num)
		if text != "" {
			return text
		}
	}
	return "эффективный процесс"
}

// PrepSlot — предложный падеж вместе с предлогом.
//
// Предлог «о» сливается с началом слова: «об идее», но «о поле».
// Слияние зависит от формы, поэтому оно считается на лету.
type PrepSlot struct {
	Prep string
}

func (p *PrepSlot) Prepare(c *morph.Ctx) {}

func (p *PrepSlot) Render(c *morph.Ctx) string {
	for i := 0; i < 40; i++ {
		lemma := randomNoun(c)
		noun, ok := c.L.Noun(lemma)
		if !ok {
			continue
		}
		num := morph.NumSg
		if c.R.Intn(100) < 30 {
			num = morph.NumPl
		}
		if noun.Pl {
			num = morph.NumPl
		}
		form, ok := c.NounForm(lemma, morph.CasePrep, num)
		if !ok {
			continue
		}
		return mergePrep(p.Prep, form)
	}
	return p.Prep + " процессе"
}

// mergePrep склеивает предлог с формой: «о» перед «и» и «у» становится «об».
func mergePrep(prep, form string) string {
	if prep != "о" {
		return prep + " " + form
	}
	low := []rune(strings.ToLower(form))
	if len(low) == 0 {
		return prep + " " + form
	}
	first := low[0]
	second := rune(0)
	if len(low) > 1 {
		second = low[1]
	}
	if first == 'и' || first == 'у' ||
		((first == 'е' || first == 'ё') && (second == 'и' || second == 'у')) {
		return "об " + form
	}
	return "о " + form
}

// VP — сказуемое с дополнением: глагол и существительное.
//
// Падеж дополнения задаёт глагол: «минимизировать издержки» (вин.),
// «достигать цели» (род.). Поэтому глагол выбирается первым.
//
// Obj: "" — падеж по валентности глагола; иначе фиксированный.
type VP struct {
	Person int
	Number string
	Tense  string
	Obj    string
	// Decl: дополнение — несклоняемое слово (митап, хайп, ретеншн).
	Decl bool
}

func (v *VP) Prepare(c *morph.Ctx) {}

func (v *VP) Render(c *morph.Ctx) string {
	num := v.Number
	gender := morph.GenM
	if c.Subject != nil {
		if num == "" || num == "auto" {
			num = c.Subject.Number
		}
		gender = c.Subject.Gender
	}
	if num == "" {
		num = morph.NumSg
	}
	for i := 0; i < 60; i++ {
		lemma := randomVerb(c)
		form, ok := c.VerbForm(lemma, v.Tense, num, gender, v.Person)
		if !ok {
			continue
		}
		if v.Decl {
			return form + " " + renderDecl(c)
		}
		objCase := v.Obj
		if objCase == "" {
			objCase = c.L.ObjCase(lemma)
		}
		return form + " " + renderObject(c, objCase)
	}
	return "оптимизирует процессы"
}

// renderDecl подбирает несклоняемое слово.
func renderDecl(c *morph.Ctx) string {
	words := declPool(c)
	if len(words) == 0 {
		return "митап"
	}
	return words[c.R.Intn(len(words))]
}

// renderObject подбирает существительное в нужном падеже.
func renderObject(c *morph.Ctx, cs string) string {
	for i := 0; i < 40; i++ {
		lemma := randomNoun(c)
		noun, ok := c.L.Noun(lemma)
		if !ok {
			continue
		}
		num := morph.NumSg
		if c.R.Intn(100) < 35 {
			num = morph.NumPl
		}
		if noun.Pl {
			num = morph.NumPl
		}
		if f, ok := c.NounForm(lemma, cs, num); ok {
			return f
		}
	}
	return "процессы"
}

// VerbSlot — глагол.
//
// Person: 1 («мы») или 3. Tense: "pres" или "past".
// Number: "auto" — по подлежащему.
type VerbSlot struct {
	Person int
	Number string
	Tense  string
}

func (v *VerbSlot) Prepare(c *morph.Ctx) {}

func (v *VerbSlot) Render(c *morph.Ctx) string {
	num := v.Number
	gender := morph.GenM
	if c.Subject != nil {
		if num == "" || num == "auto" {
			num = c.Subject.Number
		}
		gender = c.Subject.Gender
	}
	if num == "" {
		num = morph.NumSg
	}
	for i := 0; i < 60; i++ {
		lemma := randomVerb(c)
		if f, ok := c.VerbForm(lemma, v.Tense, num, gender, v.Person); ok {
			return f
		}
	}
	return "оптимизирует"
}

// InfSlot — инфинитив глагола.
type InfSlot struct{}

func (i *InfSlot) Prepare(c *morph.Ctx) {}
func (i *InfSlot) Render(c *morph.Ctx) string {
	for i := 0; i < 60; i++ {
		lemma := randomVerb(c)
		if f, ok := c.VerbForm(lemma, "inf", morph.NumSg, "", 1); ok {
			return f
		}
	}
	return "оптимизировать"
}

// AdvSlot — наречие.
type AdvSlot struct{}

func (a *AdvSlot) Prepare(c *morph.Ctx) {}
func (a *AdvSlot) Render(c *morph.Ctx) string {
	advs := advPool(c)
	if len(advs) == 0 {
		return "активно"
	}
	return advs[c.R.Intn(len(advs))]
}

// TitleSlot — должность с подразделением в родительном падеже:
// «генеральный директор департамента стратегического планирования».
type TitleSlot struct{}

func (t *TitleSlot) Prepare(c *morph.Ctx) {}

func (t *TitleSlot) Render(c *morph.Ctx) string {
	if len(c.L.Titles) == 0 || len(c.L.Departments) == 0 {
		return "генеральный директор"
	}
	title := c.L.Titles[c.R.Intn(len(c.L.Titles))]
	dept := c.L.Departments[c.R.Intn(len(c.L.Departments))]
	return title.Ru + " департамента " + dept.Ru
}

// Frame — предложение: последовательность кусков.
type Frame []Part

// Render готовит подлежащие, потом отрисовывает всё по порядку.
func (f Frame) Render(c *morph.Ctx) string {
	for _, p := range f {
		p.Prepare(c)
	}
	out := ""
	for _, p := range f {
		s := p.Render(c)
		if s == "" {
			continue
		}
		if out != "" && !strings.HasPrefix(s, ",") && !strings.HasPrefix(s, ";") {
			out += " "
		}
		out += s
	}
	return out
}

// Starters — зачинки, с которых может начинаться предложение. Если часть
// начинается с зачинки, перед ней связку не ставим.
var Starters = []string{
	"таким образом,",
	"в то же время",
	"тем не менее,",
	"вместе с тем,",
	"в итоге",
}
