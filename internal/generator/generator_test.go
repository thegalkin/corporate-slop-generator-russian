package generator

import (
	"regexp"
	"strings"
	"testing"

	"github.com/thegalkin/corporate-slop-generator-russian/internal/lexicon"
	"github.com/thegalkin/corporate-slop-generator-russian/internal/morph"
)

// базовые инварианты текста
var (
	doubleSpace = regexp.MustCompile(`  +`)
	spaceBefore = regexp.MustCompile(` [,.;:]`)
	badStart    = regexp.MustCompile(`^[ ,.;]`)
)

func TestGeneratorInvariants(t *testing.T) {
	g, err := New(42)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2000; i++ {
		s := g.Sentence()
		if s == "" {
			t.Fatalf("пустое предложение")
		}
		if doubleSpace.MatchString(s) {
			t.Fatalf("двойной пробел: %q", s)
		}
		if spaceBefore.MatchString(s) {
			t.Fatalf("пробел перед знаком: %q", s)
		}
		if badStart.MatchString(s) {
			t.Fatalf("плохое начало: %q", s)
		}
		if !strings.HasSuffix(s, ".") {
			t.Fatalf("нет точки в конце: %q", s)
		}
	}
}

// согласованность прилагательного с существительным: слотовый слой берёт
// форму из таблицы, поэтому проверяем, что форма вообще существует
// для каждого сочетания «падеж × род × число».
func TestAdjectiveFormsComplete(t *testing.T) {
	l, err := lexicon.Load()
	if err != nil {
		t.Fatal(err)
	}
	for lemma, a := range l.Adjectives {
		for _, cs := range []string{morph.CaseNom, morph.CaseGen, morph.CaseDat, morph.CaseAcc, morph.CaseIns, morph.CasePrep} {
			for _, num := range []string{morph.NumSg, morph.NumPl} {
				if num == morph.NumPl {
					if _, ok := a.Forms["a:"+cs+":pl:-"]; !ok {
						t.Fatalf("%s: нет формы a:%s:pl:-", lemma, cs)
					}
					continue
				}
				for _, g := range []string{"m", "f", "n"} {
					key := "a:" + cs + ":sg:" + g + ":-"
					if _, ok := a.Forms[key]; !ok {
						t.Fatalf("%s: нет формы %s", lemma, key)
					}
				}
			}
		}
	}
}

// у существительного должны быть все падежи в обоих числах.
func TestNounFormsComplete(t *testing.T) {
	l, err := lexicon.Load()
	if err != nil {
		t.Fatal(err)
	}
	for lemma, n := range l.Nouns {
		for _, cs := range []string{morph.CaseNom, morph.CaseGen, morph.CaseDat, morph.CaseAcc, morph.CaseIns, morph.CasePrep} {
			for _, num := range []string{morph.NumSg, morph.NumPl} {
				if n.Pl && num == morph.NumSg {
					continue
				}
				key := "n:" + cs + ":" + num
				if _, ok := n.Forms[key]; !ok {
					t.Fatalf("%s: нет формы %s", lemma, key)
				}
			}
		}
	}
}

// у глагола должны быть все формы, нужные генератору.
func TestVerbFormsComplete(t *testing.T) {
	l, err := lexicon.Load()
	if err != nil {
		t.Fatal(err)
	}
	need := []string{"pres:sg:1", "pres:sg:3", "pres:pl:1", "pres:pl:3",
		"past:sg:m", "past:sg:f", "past:sg:n", "past:pl:pl", "inf"}
	for lemma, v := range l.Verbs {
		for aspect, forms := range v.Aspects {
			for _, k := range need {
				if _, ok := forms[k]; !ok {
					t.Fatalf("%s [%s]: нет формы %s", lemma, aspect, k)
				}
			}
		}
	}
}
