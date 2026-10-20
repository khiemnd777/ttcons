package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestReadManifestRejectsUnexpectedOrDuplicateObject(t *testing.T) {
	build := func() []string {
		keys := make([]string, 0, 23)
		for i := range 13 {
			keys = append(keys, fmt.Sprintf("evidence/project/%d.png", i))
		}
		for i := range 9 {
			keys = append(keys, fmt.Sprintf("workflow-form-pdfs/project/%d.pdf", i))
		}
		return append(keys, "workflow-signatures/project/0.png")
	}
	for _, test := range []struct {
		name   string
		change func([]string)
		valid  bool
	}{
		{name: "exact archive manifest", valid: true},
		{name: "duplicate", change: func(keys []string) { keys[1] = keys[0] }},
		{name: "wrong family", change: func(keys []string) { keys[0] = "project-documents/project/0.png" }},
		{name: "path traversal", change: func(keys []string) { keys[0] = "evidence/../0.png" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			keys := build()
			if test.change != nil {
				test.change(keys)
			}
			got, err := readManifest(strings.NewReader(strings.Join(keys, "\n") + "\n"))
			if test.valid {
				if err != nil || len(got) != 23 {
					t.Fatalf("valid manifest rejected: count=%d err=%v", len(got), err)
				}
			} else if err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}
