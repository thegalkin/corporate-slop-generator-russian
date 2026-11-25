BIN := bin/cbsg
PY := .venv/bin/python

.PHONY: all build test check lexicon run clean venv install uninstall

all: build

venv:
	python3 -m venv .venv
	.venv/bin/pip install -q pymorphy3 pyyaml

build:
	go build -o $(BIN) ./cmd/cbsg

# поставить в ~/.local/bin (он обычно уже в PATH)
install: build
	install -Dm755 $(BIN) $(HOME)/.local/bin/cbsg
	@echo "установлено: $$(command -v cbsg)"

uninstall:
	rm -f $(HOME)/.local/bin/cbsg

test:
	go test ./...

# пересобрать таблицы форм из OpenCorpora
lexicon:
	$(PY) tools/build_lexicon.py

# независимая проверка грамматики вывода
check: build
	$(PY) tools/check_grammar.py -n 5000

run: build
	$(BIN) -n 5

clean:
	rm -rf bin
