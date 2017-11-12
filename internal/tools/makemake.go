// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// makemake extracts the build and test info from the pom.xml to generate
// a Makefile capable to build and test all the grammars with Go.
package main

import (
	"bramp.net/antlr4/internal"

	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var IGNORE_PATHS = []string{"/antlr4/examples/",
	"/CSharpSharwell/", "/Python/", "/CSharp/", "/JavaScript/", "/two-step-processing/",
	"/python3-js/", "/python3-py/", "/python3-ts/",
	".TypeScriptTarget.", ".JavaScriptTarget.", ".PythonTarget.",
	"/LexBasic.g4", "ecmascript/ECMAScript.g4"}

const GRAMMARS_ROOT = "grammars-v4"

// MAKEFILE is the template used to build the Makefile.
// It expects to be executed with a templateData
const MAKEFILE = `# Copyright 2017 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#
# Do not edit this file, it is generated by makemake.go
#
MAKEFLAGS += --no-builtin-rules --warn-undefined-variables

.PHONY: all clean rebuild test
.DEFAULT_GOAL := all
.SILENT:
.DELETE_ON_ERROR:
.SUFFIXES:

ANTLR_BIN := $(PWD)/.bin/antlr-4.7-complete.jar
ANTLR_URL := http://www.antlr.org/download/antlr-4.7-complete.jar
ANTLR := java -jar $(ANTLR_BIN) -Dlanguage=Go -listener -no-visitor

GRAMMARS :={{ range $name, $grammar := .Grammars }} {{$name}}{{ end }}

LANG_COLOR = \033[0;36m
NO_COLOR   = \033[m

XLOG=printf "| %s  | $(LANG_COLOR)%-15s$(NO_COLOR) | %-75s |\n" "❌"
LOG=printf "| %s  | $(LANG_COLOR)%-15s$(NO_COLOR) | %-75s |\n" "✅"

# This is the default target (which cleans and rebuilds everything)
all: Makefile
	$(MAKE) -k -j2 rebuild 2> /dev/null

clean:
	-rm -r $(GRAMMARS) 2> /dev/null

rebuild: $(GRAMMARS)

$(GRAMMARS): $(ANTLR_BIN)
	$(LOG) "$@"

$(ANTLR_BIN):
	mkdir -p .bin
	curl -o $@ $(ANTLR_URL)

Makefile: internal/tools/makemake.go grammars-v4
	go run internal/tools/makemake.go

grammars-v4:
	git submodule init
	git submodule update

# Define build as a "function" so it can be called over and over
# This could have been a standard target, but each Grammar depends on
# and creates a slightly different set of named files. This makes it
# difficult to have one file. This is the best hack that keeps Make
# working correctly.
BUILD=sh -c '\
	basedir=$$PWD; \
	errors=$$0/$$(basename $$1).log; \
	mkdir -p $$0; \
	pushd $$(dirname $$1) > /dev/null; \
	$(ANTLR) -package $$0 $$(basename $$1) -o $$basedir/$$0 > $$basedir/$$errors 2>&1; \
	RET=$$?; \
	popd > /dev/null; \
	if [ $$RET -ne 0 ]; then \
		$(XLOG) "$$0" "antlr: $$(tail -n 1 $$errors)"; \
		exit $$RET; \
	fi; \
	shift; shift; \
	go build ./$$0 >> $$errors 2>&1; \
	RET=$$?; \
	if [ $$RET -ne 0 ]; then \
		$(XLOG) "$$0" "build: $$(tail -n 1 $$errors)"; \
		exit $$RET; \
	fi;'

TEST=sh -c '\
	errors=$$0/$$0.log; \
	go run internal/tools/make.go test $$0 "$$@" >> $$errors 2>&1; \
	RET=$$?; \
	if [ $$RET -ne 0 ]; then \
		$(XLOG) "$$0" "maketest: $$(tail -n 1 $$errors)"; \
		exit $$RET; \
	fi; \
	go test -timeout 10s -count 3 ./$$0 >> $$errors 2>&1; \
	RET=$$?; \
	if [ $$RET -ne 0 ]; then \
		$(XLOG) "$$0" "test: $$(tail -n 1 $$errors)"; \
		exit $$RET; \
	fi;'

%/doc.go:
	errors=$*/$*.log; \
	go run internal/tools/make.go doc $* >> $$errors 2>&1; \
	RET=$$?; \
	if [ $$RET -ne 0 ]; then \
		$(XLOG) "$*" "makedoc: $$(tail -n 1 $$errors)"; \
		exit $$RET; \
	fi;

{{ range $name, $grammars := .Grammars }}
{{ $name }}: {{ $name }}/{{ $name }}_test.go {{ $name }}/doc.go
{{ $name }} {{ $name }}/{{ $name }}_test.go: {{ range $i, $grammar := $grammars }}{{ Join (FilePathJoin $name $grammar.GeneratedFilenames) " " }} {{ end }}
{{ $name }}/doc.go: {{ $name }}/{{ $name }}_test.go
{{- range $i, $grammar := $grammars }}
{{/* Create a literal target, to ensure all targets are built concurrently */}}
{{ (Join (FilePathJoin "%" $grammar.GeneratedFilenames) " ") }}: {{ $grammar.Filename }} {{ (Join (FilePathJoin $name $grammar.DependentFilenames) " ") }}
	${BUILD} {{ $name }} {{ $grammar.Filename }} {{ (Join (FilePathJoin $name $grammar.GeneratedFilenames) " ") }}
{{- end }}
%/{{ $name }}_test.go: {{ range $i, $grammar := $grammars }}{{ Join (FilePathJoin $name $grammar.GeneratedFilenames) " " }} {{ end }}
	${TEST} {{ $name }} {{ Pom $grammars }} {{ range $i, $grammar := $grammars }}{{ $grammar.Filename }} {{ end }}
{{ end }}
`

type templateData struct {
	Grammars map[string][]*internal.Grammar
}

func containAny(name string, any []string) bool {
	for _, a := range any {
		if strings.Contains(name, a) {
			return true
		}
	}
	return false
}

func main() {
	g4s := make(map[string][]*internal.Grammar)

	// Find all g4 files
	err := filepath.Walk(GRAMMARS_ROOT, func(path string, info os.FileInfo, err error) error {

		// Ignore some paths
		if containAny(path, IGNORE_PATHS) {
			return nil
		}

		if err == nil && strings.HasSuffix(path, ".g4") {
			g4, err := internal.ParseG4(path)
			if err != nil {
				return err
			}

			name := strings.ToLower(g4.Name)
			if len(g4s[name]) > 0 {
				log.Fatalf("Multiple g4 files generate the same grammar:\n%q: %q %q", name, g4s[name][0], path)
			}

			g4s[name] = append(g4s[name], g4)
		}

		return nil
	})
	if err != nil {
		log.Fatalf("failed to walk: %s", err)
	}

	// Merge Parser and Lexer into same package
	for name, files := range g4s {
		if strings.HasSuffix(name, "parser") {
			name = strings.TrimSuffix(name, "parser")
			parser := name + "parser"
			lexer := name + "lexer"

			if _, found := g4s[lexer]; found {
				if _, found := g4s[name]; found {
					log.Fatalf("Can not merge Parser and Lexer for: %q", name)
				}

				// Merge
				g4s[name] = []*internal.Grammar{files[0], g4s[lexer][0]}
				delete(g4s, parser)
				delete(g4s, lexer)
			} else {
				if _, found := g4s[name]; found {
					log.Fatalf("Can not shorten Parser name: %q", name)
				}

				g4s[name] = g4s[parser]
				delete(g4s, parser)
			}
		}
	}

	// Final parse for Lexers that have mismatching Parser
	for name, files := range g4s {
		if strings.HasSuffix(name, "lexer") {
			name = strings.TrimSuffix(name, "lexer")
			lexer := name + "lexer"
			if len(g4s[name]) > 1 {
				log.Fatalf("Can not shorten Lexer name: %q", name)
			}
			g4s[name] = []*internal.Grammar{g4s[name][0], files[0]}
			delete(g4s, lexer)
		}
	}

	data := templateData{
		Grammars: g4s,
	}

	funcs := template.FuncMap{
		"Join": strings.Join,
		"FilePathJoin": func(prefix string, arr []string) []string {
			for i, a := range arr {
				arr[i] = filepath.Join(prefix, a)
			}
			return arr
		},
		"Pom": func(grammars []*internal.Grammar) string {
			// Returns the path to the pom file, that defines this grammar
			// TODO(bramp) This makes some bad assumptions, that seem to work
			if len(grammars) < 1 {
				panic("Pom needs atleast one grammar, zero given.")
			}

			// Only look at the first grammar (for no reason other than we're lazy).
			g := grammars[0]
			filename := filepath.Join(filepath.Dir(g.Filename), "pom.xml")

			// HACK to work around one exception where the pom.xml is not in the same directory as the .g4
			return strings.Replace(filename, "/Go/", "/", 1)
		},
	}

	makeTemplate := template.Must(template.New("makefile").Funcs(funcs).Parse(MAKEFILE))

	out, err := os.Create("Makefile")
	if err != nil {
		log.Fatalf("failed to create Makefile: %s", err)
	}

	if err := makeTemplate.Execute(out, data); err != nil {
		log.Fatalf("failed to generate Makefile: %s", err)
	}
}
