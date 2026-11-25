// Package generator собирает предложения русского корпоративного булшита.
//
// Устройство повторяет оригинал CBSG: случайная грамматика собирает
// предложение из предложений, предложения соединяются связками, получается
// абзац. Отличие — в том, что здесь падеж, число и род согласованы
// морфологическим слоем, а не подставляются текстом.
package generator

import (
	"fmt"
	"math/rand"
	"strings"
	"unicode"

	"github.com/thegalkin/corporate-slop-generator-russian/internal/grammar"
	"github.com/thegalkin/corporate-slop-generator-russian/internal/lexicon"
	"github.com/thegalkin/corporate-slop-generator-russian/internal/morph"
)

// Generator — генератор предложений.
type Generator struct {
	L     *lexicon.Lexicon
	R     *rand.Rand
	pools *morph.Pools
}

// New создаёт генератор. seed задаёт воспроизводимость.
func New(seed int64) (*Generator, error) {
	l, err := lexicon.Load()
	if err != nil {
		return nil, err
	}
	return &Generator{
		L:     l,
		R:     rand.New(rand.NewSource(seed)),
		pools: morph.BuildPools(l),
	}, nil
}

// Seed пересевает генератор.
func (g *Generator) Seed(seed int64) { g.R = rand.New(rand.NewSource(seed)) }

func (g *Generator) ctx() *morph.Ctx {
	return &morph.Ctx{L: g.L, R: g.R, Pools: g.pools}
}

// Clause — одна законченная часть предложения.
func (g *Generator) Clause() string {
	f := grammar.Frames[g.R.Intn(len(grammar.Frames))]
	return f.Render(g.ctx())
}

// Sentence — предложение: одна или несколько частей, связанных связками.
func (g *Generator) Sentence() string {
	n := 1
	switch g.R.Intn(100) {
	case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29:
		n = 2
	case 30, 31, 32, 33, 34, 35, 36, 37, 38, 39:
		n = 3
	}
	out := ""
	for i := 0; i < n; i++ {
		clause := g.Clause()
		if i > 0 {
			// зачинка в середине предложения смотрится плохо:
			// перебираем рамку, пока не найдём обычную.
			for tries := 0; tries < 12 && startsWithStarter(clause); tries++ {
				clause = g.Clause()
			}
			out += grammar.Connectors[g.R.Intn(len(grammar.Connectors))] + " "
		}
		out += clause
	}
	return capitalize(out) + "."
}

// startsWithStarter: зачинка и связка вместе не нужны.
func startsWithStarter(clause string) bool {
	for _, s := range grammar.Starters {
		if strings.HasPrefix(clause, s) {
			return true
		}
	}
	return false
}

// Paragraph — несколько предложений подряд.
func (g *Generator) Paragraph(n int) string {
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, g.Sentence())
	}
	return strings.Join(parts, " ")
}

// Workshop — поток предложений: столько, сколько попросят.
func (g *Generator) Workshop(n int) string {
	return g.Paragraph(n)
}

// Report — «финансовый отчёт»: абзацы с заголовками.
func (g *Generator) Report(paragraphs int) string {
	var b strings.Builder
	for i := 0; i < paragraphs; i++ {
		b.WriteString(capitalize(g.Clause()))
		b.WriteString(". ")
	}
	return strings.TrimSpace(b.String())
}

// Title — одна должность с подразделением.
func (g *Generator) Title() string {
	c := g.ctx()
	return capitalize((&grammar.TitleSlot{}).Render(c)) + "."
}

// capitalize поднимает первую букву.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// Stats — счётчики для отчёта.
type Stats struct {
	Sentences int
	Words     int
}

// Summarize печатает краткую сводку.
func (g *Generator) Summarize(text string) string {
	return fmt.Sprintf("%d предложений, %d слов", strings.Count(text, "."), len(strings.Fields(text)))
}
