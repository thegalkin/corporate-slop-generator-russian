# Corporate Slop Generator — Russian

**Генератор корпоративной тарабарщины на русском языке.**
Предложения без ошибок склонения, спряжения и согласования.

```
$ cbsg -n 3
Мы рефреймим импульс, а команда инкубирует опыт, в результате совет директоров оптимизирует процессы.
Мобильное подразделение результативно нивелирует контракты.
Интеллектуальный капитал монетизирует платформу.
```

---

## English overview

A port of the [Corporate Bullshit Generator](https://github.com/zertovitch/cbsg)
(Ada, MIT) to Russian. The original generates English sentences like
*"Our turn-key branding strategy drives corporate, scalable, wide-ranging and
profit-oriented idiosyncratic incentives"*; this one does the same in Russian —
**with correct grammar**: case, number, gender, verb aspect and agreement are
computed, not pasted.

* Every noun carries its full paradigm (6 cases × 2 numbers), every adjective
  agrees with its noun in case, number and gender, every verb agrees with its
  subject.
* Case is a property of the *frame*, not of the word: `«в рамках»` demands the
  genitive, `«к»` the dative, `«о поле»` stays `о`, but `«об идее»` becomes `об`.
* The morphology tables are extracted from [OpenCorpora](https://opencorpora.ru)
  at build time and shipped as a static JSON — the binary itself has **zero
  dependencies**.

| | |
|---|---|
| Language | Go 1.27+, no runtime dependencies |
| Speed | 20 000 sentences (190 000 words) in 0.14 s |
| Vocabulary | 206 nouns, 161 adjectives, 131 verbs, 84 adverbs, 30 compounds |
| Tests | `go test ./...` + an independent grammar checker |

---

## Зачем это нужно

Оригинал CBSG держит в грамматике единственную переменную — **число**:

```ada
type Plurality is (Singular, Plural);
```

Вся морфология — три функции общим объёмом в тридцать строк
(`Make_Possible_Plural`, `Build_Plural_Verb`, `Add_Indefinite_Article`).
Перенести это на русский нельзя: здесь нужно согласовать

* существительное с прилагательным — по падежу, числу и роду;
* глагол с подлежащим — по лицу, числу и роду (в прошедшем времени);
* существительное с предлогом — по падежу;
* существительное с глаголом — по падежу («достигать **цели**», а не «достигать цель»).

Поэтому порт устроен иначе: **падеж — свойство рамки, а не слова**.

```go
// внутренний/grammar/frames.go
{Lit("в рамках"), noun(morph.CaseGen), subj("team"), vp()},
```

Слот `{noun gen}` просит родительный падеж, `{vp}` выбирает глагол и сам
решает, какой падеж будет у дополнения.

---

## Сборка и запуск

Нужен только Go. Ничего больше.

```bash
git clone https://github.com/thegalkin/corporate-slop-generator-russian
cd corporate-slop-generator-russian
make build          # бинарник в bin/cbsg
make run            # пять предложений
```

Поставить в `~/.local/bin` (обычно уже в PATH) — одной командой:

```bash
make install        # -> ~/.local/bin/cbsg
```

Или без make:

```bash
go build -o bin/cbsg ./cmd/cbsg
./bin/cbsg -n 10
```

### Команды

```bash
cbsg                    # одно предложение
cbsg -n 10              # десять предложений
cbsg -mode report       # отчёт из абзацев
cbsg -mode title        # должность с подразделением
cbsg -mode paragraph    # абзац
cbsg -seed 42           # воспроизводимый результат
cbsg -version
```

Примеры вывода лежат в [`demo/`](demo/).

### Continuous integration

Конфигурация CI — в [`ci/ci.yml`](ci/ci.yml). Она не опубликована как
workflow, потому что для этого нужен OAuth-скоуп `workflow`:

```bash
gh auth refresh -h github.com -s workflow
mkdir -p .github/workflows && cp ci/ci.yml .github/workflows/ci.yml
git add .github/workflows/ci.yml && git commit -m "ci: add workflow" && git push
```

### Docker

```bash
docker build -t cslop .
docker run --rm cslop -n 20
```

---

## Сборка: инструкция для агентов

Этот раздел — для ИИ-агентов, которые правят репозиторий или пользуются
бинарником.

### Собрать и проверить

```bash
make build      # go build -o bin/cbsg ./cmd/cbsg
make test       # go test ./...
make check      # независимая проверка грамматики: 5000 предложений
make lexicon    # пересобрать таблицы форм из OpenCorpora
```

`make check` требует Python-окружения (`make venv` его создаёт: `pymorphy3`,
`pyyaml`). Оно нужно **только** для сборки словаря и для проверки грамматики —
сам бинарник зависимостей не имеет.

### Если нужно добавить или починить слово

1. Правь `tools/lexicon.yaml` (это единственный источник словаря).
   Слова идут по секциям: `nouns`, `adjectives`, `verbs`, `adverbs`,
   `indeclinable`, `phrases`, `actors`, `titles`, `departments`, `loans`.
   Ключ — русское слово, значение — английский оригинал (для справки).
   Ключи с пробелом пишутся через подчёркивание: `рабочая_группа`.

2. Пересобери таблицы форм:

   ```bash
   make lexicon
   ```

   Сборщик сам проверит: слово есть в словаре, род определён, собраны все
   падежи в обоих числах, И.п. ед.ч. совпадает с леммой, В.п. = Р.п. у
   одушевлённых и В.п. = И.п. у неодушевлённых мужского рода.
   Если что-то не так — сборка упадёт с точным списком проблем.

3. Проверь грамматику вывода:

   ```bash
   make check
   ```

   Валидатор разбирает **готовый текст** анализатором pymorphy3 и ищет
   нарушения согласования. Он смотрит на вывод, а не на внутренние таблицы,
   поэтому ловит то, что таблицы пропускают.

4. Если леммы нет в OpenCorpora — положи её в секцию `loans` и задай
   парадигму руками; сборщик подтвердит её round-trip'ом через анализатор.
   Либо возьми парадигму у существующего слова: `template: хороший`,
   `from: хорош`, `to: ширш` — и получишь гарантированно верные формы.

### Правила, которые нельзя нарушать

* **Не править сгенерированный `internal/lexicon/data/lexicon.json` руками.**
  Это генерируемый файл. Правь `tools/lexicon.yaml` и пересобирай.
* **Не добавлять в рантайм зависимость на Python.** pymorphy3 нужен только
  на этапе сборки; в рантайме бинарник обязан собираться `CGO_ENABLED=0`
  без внешних пакетов.
* **Не ломать порядок заполнения слотов.** Прилагательное согласуется с
  *ближайшим следующим* существительным, поэтому существительные в рамке
  отрисовываются раньше прилагательных. Отсюда — порядок частей в
  `RenderParts`.
* **Слоты рамок живут в общей таблице.** У `Subj` и `SubjAdj` есть
  изменяемое состояние, поэтому `Prepare` обязан его сбрасывать — иначе
  прилагательное «залипнет» от прошлого предложения.

### Полезные команды

```bash
./bin/cbsg -n 20000 -seed 1 -quiet | wc -w     # производительность
go test ./... -run TestGeneratorInvariants       # инварианты текста
./bin/cbsg -mode title -seed 1                   # посмотреть должности
```

---

## Как устроено

```
cmd/cbsg                 командная строка
internal/lexicon          таблицы форм + встроенный lexicon.json
internal/morph            лемма -> форма, согласование прилагательного
internal/grammar          рамки предложений: каждый слот знает свой падеж
internal/generator        сборка предложений, абзацев, отчётов
tools/lexicon.yaml        источник словаря
tools/build_lexicon.py    сборка таблиц форм из OpenCorpora
tools/check_grammar.py    независимая проверка грамматики вывода
demo/                    примеры вывода
```

Ключи форм:

| что | ключ | пример |
|---|---|---|
| существительное | `n:<падеж>:<число>` | `n:gen:pl` → «стратегий» |
| прилагательное | `a:<падеж>:<число>:<род>:<одушевлённость>` | `a:acc:sg:f:-` → «эффективной» |
| глагол | `pres:<число>:<лицо>`, `past:<число>:<род>` | `pres:pl:3` → «оптимизируют» |

Во множественном числе русский язык снимает род, поэтому ключ прилагательного
короче: `a:<падеж>:pl:-`.

---

## Словарь

Леммы хранятся в `tools/lexicon.yaml` рядом с английским оригиналом. Формы
вытаскиваются из OpenCorpora офлайн:

```bash
make lexicon
```

```
лексикон: 206 сущ., 161 прил., 131 глаг., 84 нареч., 45 нескл., 5 ручных, 30 фраз
всё чисто.
```

Несклоняемые транслитерации (митап, хайп, ретеншн, коммитмент, перформанс)
перечислены отдельно: в русском они не склоняются.

В словаре есть слово **«ширший»**, и означает оно «быстрее», а не «шире»:

```
Команды шире позиционируют стратегию.
Команды ширше переориентируют стратегию.
```

Парадигма у него не выдумана — взята у настоящего прилагательного «хороший»
с подменой основы, поэтому формы гарантированно верны.

---

## Проверка грамматики

`tools/check_grammar.py` — внешняя проверка. Она генерирует текст, разбирает
его анализатором pymorphy3 и ищет нарушения согласования. Проверка
неоднозначна: у русского слова часто несколько разборов («ценности» — это и
род. ед. ж. р. от «ценность», и им. мн. ч. от «ценность»), поэтому слово
описывается множеством разборов, а нарушение — это когда множества
несовместимы.

```bash
$ make check
проверено предложений: 5000
нарушений не найдено.
```

---

## Лицензии и благодарности

* Код — MIT, как у оригинала ([Gautier de Montmollin](https://github.com/zertovitch/cbsg)).
* Python-порядок оригинала — [bpgs/corporate_bullshit](https://github.com/bpgs/corporate_bullshit)
  (GPL-3.0) использован как образец алгоритма, код не копировался.
* Таблицы форм собраны из словаря [OpenCorpora](https://opencorpora.ru)
  (CC-BY-SA 3.0) — при распространении лексикона укажите источник.
* `pymorphy3` (MIT) нужен только на этапе сборки.

```
MIT License

Copyright (c) 2026 thegalkin

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
