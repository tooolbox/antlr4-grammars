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
	"bramp.net/antlr4-grammars/internal"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

const GRAMMARS_ROOT = "grammars-v4"

// IGNORE these projects
var IGNORE = []string{
	"objc",      // Is actually two subprojects, needs splitting out.
	"swift-fin", // The g4 files are nested under a src/main/... directory, which we can't handle.
}

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
MAKEFLAGS += --no-builtin-rules

.PHONY: all antlr clean rebuild test
.SILENT:
.DELETE_ON_ERROR:
.SUFFIXES:

ANTLR_BIN := $(PWD)/.bin/antlr-4.7-complete.jar
ANTLR_URL := http://www.antlr.org/download/antlr-4.7-complete.jar
ANTLR_ARGS := -Dlanguage=Go -visitor

GRAMMARS := {{ Join .Grammars " " }}

LANG_COLOR = \033[0;36m
NO_COLOR   = \033[m

# This is the default target
rebuild: antlr test

all:
	go run makemake.go
	make clean
	make -k -j2 rebuild 2> /dev/null

clean:
	@rm -r $(GRAMMARS) 2> /dev/null || true

antlr: $(ANTLR_BIN)
$(ANTLR_BIN):
	mkdir -p .bin
	curl -o $@ $(ANTLR_URL)

test: {{ range $name, $project := .Projects -}}{{ $name }}/{{ $project.FilePrefix }}_test.go {{ end }}

{{ range $name, $project := .Projects -}}
{{ $genfiles := (Join (index $.GeneratedFiles $name) " ") }}
{{ $testfile := (Concat $name "/" $project.FilePrefix "_test.go") }}
{{ $name }}: {{ $testfile }}
{{ $genfiles }}: {{ Join $project.Includes " " }}
{{ $testfile }}: {{ $genfiles }}
{{- end }}

%_lexer.go %_parser.go:
	lang=$$(dirname $@); \
	errors=$$lang/$$(basename $*).errors; \
	mkdir -p $$lang; \
	pushd $$(dirname $<) > /dev/null; \
	java -jar $(ANTLR_BIN) $(ANTLR_ARGS) -package $$lang $(notdir $^) -o ../../$$lang > ../../$$errors 2>&1; \
	RET=$$?; \
	popd > /dev/null; \
	if [ $$RET -ne 0 ]; then \
		printf "| %s  | $(LANG_COLOR)%-15s$(NO_COLOR) | %-75s |\n" "❌" "$$lang" "antlr: $$(tail -n 1 $$errors)"; \
		rm $$lang/*.go > /dev/null 2>&1 || true; \
		exit $$RET; \
	fi; \
	shopt -s nullglob; \
	go build $*_*.go $*parser_*.go >> $$errors 2>&1; \
	RET=$$?; \
	if [ $$RET -ne 0 ]; then \
		printf "| %s  | $(LANG_COLOR)%-15s$(NO_COLOR) | %-75s |\n" "❌" "$$lang" "build: $$(tail -n 1 $$errors)"; \
		exit $$RET; \
	fi;

%_test.go:
	lang=$$(dirname $@); \
	errors=$$lang/$$(basename $*).errors; \
	go run maketest.go $$lang >> $$errors 2>&1; \
	RET=$$?; \
	if [ $$RET -ne 0 ]; then \
		printf "| %s  | $(LANG_COLOR)%-15s$(NO_COLOR) | %-75s |\n" "❌" "$$lang" "maketest: $$(tail -n 1 $$errors)"; \
		exit $$RET; \
	fi; \
	go test -timeout 10s ./$$lang >> $$errors 2>&1; \
	RET=$$?; \
	if [ $$RET -ne 0 ]; then \
		printf "| %s  | $(LANG_COLOR)%-15s$(NO_COLOR) | %-75s |\n" "❌" "$$lang" " test: $$(tail -n 1 $$errors)"; \
		exit $$RET; \
	fi; \
	if [[ -s $$errors ]]; then \
		rm $$errors; \
		printf "| %s  | $(LANG_COLOR)%-15s$(NO_COLOR) | %-75s |\n" "✅" "$$lang" ""; \
	else \
		printf "| %s  | $(LANG_COLOR)%-15s$(NO_COLOR) | %-75s |\n" "⚠️" "$$lang" "$$(tail -n 1 $$errors)"; \
	fi;
`

type templateData struct {
	Grammars       []string
	Projects       map[string]*internal.Project
	GeneratedFiles map[string][]string
}

// onIgnoreList returns true if the pom file in the path is on the banned list.
func onIgnoreList(path string) bool {
	for _, ignore := range IGNORE {
		if strings.HasSuffix(path, "/"+ignore+"/pom.xml") {
			return true
		}
	}
	return false
}

func main() {
	projects := make(map[string]*internal.Project)

	err := filepath.Walk(GRAMMARS_ROOT, func(path string, info os.FileInfo, err error) error {
		if err == nil && strings.HasSuffix(path, "/pom.xml") {
			if onIgnoreList(path) {
				return nil
			}

			p, err := internal.ParsePom(path)
			if err != nil {
				return err
			}

			// Ignore pom files which don't even have the Antlr plugin
			if !p.FoundAntlr4MavenPlugin {
				return nil
			}

			if len(p.Includes) == 0 {
				log.Printf("skipping %q as it contains no grammars", path)
				return nil
			}

			// Check for g4 files not mentioned in pom
			//g4s, err := filepath.Glob(filepath.Dir(path) + "/*.g4")
			//if err != nil {
			//	return err
			//}
			//if len(g4s) != len(p.Includes) {
			//	log.Printf("mismatch g4 files:\n    *.g4: %q\nincludes: %q", g4s, p.Includes)
			//	return nil
			//}

			projects[p.PackageName()] = p
		}
		return err
	})
	if err != nil {
		log.Fatalf("failed to walk: %s", err)
	}

	var grammars []string
	for name := range projects {
		grammars = append(grammars, name)
	}
	sort.Strings(grammars)

	generatedFiles := make(map[string][]string)
	for name, project := range projects {
		var generated []string
		for _, file := range project.GeneratedFilenames() {
			// Full path to generated file
			generated = append(generated, name+"/"+file)
		}
		generatedFiles[name] = generated
		if len(generated) < 2 {
			// TODO(bramp): Actually check we have one lexer, and one parser.
			log.Fatalf("Expect atleast two generated files, only got: %q for %q", generated, name)
		}
	}

	data := templateData{
		Grammars:       grammars,
		Projects:       projects,
		GeneratedFiles: generatedFiles,
	}

	funcs := template.FuncMap{
		"Join": strings.Join,
		"Concat": func(strings ...string) string {
			results := ""
			for _, s := range strings {
				results = results + s
			}
			return results
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
