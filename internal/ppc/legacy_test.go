package ppc

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUncorrectedReferenceCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []goldenInstruction
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Assembly, func(t *testing.T) {
			word, err := Encode(tc.Assembly, Context{AllowNonConsoleInstructions: true})
			if err != nil || word != tc.Word {
				t.Fatalf("got %08x (%v), C++ %08x", word, err, tc.Word)
			}
		})
	}
}
