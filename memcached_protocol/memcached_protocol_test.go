// Package memcached_protocol_test contains tests for the memcached_protocol grammar.
// The tests should be run with the -timeout flag, to ensure the parser doesn't
// get stuck.
//
// Do not edit this file, it is generated by maketest.go
//
package memcached_protocol_test

import (
	"bramp.net/antlr4test-go/memcached_protocol"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	"path/filepath"
	"testing"
)

const MAX_TOKENS = 1000000

var examples = []string{
	"grammars-v4/memcached_protocol/examples/example1.txt",
}

func newCharStream(filename string) (antlr.CharStream, error) {
	var input antlr.CharStream
	input, err := antlr.NewFileStream(filepath.Join("..", filename))
	if err != nil {
		return nil, err
	}

	return input, nil
}

// TODO Add an Example func

func Testmemcached_protocolLexer(t *testing.T) {
	for _, file := range examples {
		input, err := newCharStream(file)
		if err != nil {
			t.Errorf("Failed to open example file: %s", err)
		}

		// Create the Lexer
		lexer := memcached_protocol.Newmemcached_protocolLexer(input)

		// Try and read all tokens
		i := 0
		for ; i < MAX_TOKENS; i++ {
			t := lexer.NextToken()
			if t.GetTokenType() == antlr.TokenEOF {
				break
			}
		}

		// If we read too many tokens, then perhaps there is a problem with the lexer.
		if i == MAX_TOKENS {
			t.Errorf("Newmemcached_protocolLexer(%q) read %d tokens without finding EOF", file, i)
		}
	}
}

func Testmemcached_protocolParser(t *testing.T) {
	for _, file := range examples {
		input, err := newCharStream(file)
		if err != nil {
			t.Errorf("Failed to open example file: %s", err)
		}

		// Create the Lexer
		lexer := memcached_protocol.Newmemcached_protocolLexer(input)
		stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

		// Create the Parser
		p := memcached_protocol.Newmemcached_protocolParser(stream)
		p.BuildParseTrees = true
		p.AddErrorListener(antlr.NewDiagnosticErrorListener(true)) // TODO Change this
		p.AddErrorListener(antlr.NewConsoleErrorListener())

		// Finally test
		p.Command_line()

		// TODO Check for errors
	}
}