package hcloverlay

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestParseCLIArgument(t *testing.T) {
	type BlockNoLabels struct {
		Foo string `hcl:"foo"`
	}
	type BlockOneLabel struct {
		Name string `hcl:"name,label"`
		Foo  string `hcl:"foo"`
	}
	type BlockTwoLabels struct {
		Type string `hcl:"type,label"`
		Name string `hcl:"name,label"`
		Foo  string `hcl:"foo"`
	}

	tests := map[string]struct {
		Config  string
		Arg     string
		Want    interface{}
		WantErr string
	}{
		"override root attribute": {
			`
			foo = "a"
			`,
			`foo=b`,
			&struct {
				Foo string `hcl:"foo"`
			}{
				Foo: "b",
			},
			``,
		},
		"new root attribute": {
			`
			foo = "a"
			`,
			`bar=b`,
			&struct {
				Foo string `hcl:"foo"`
				Bar string `hcl:"bar"`
			}{
				Foo: "a",
				Bar: "b",
			},
			``,
		},
		"unwanted root attribute": {
			`
			foo = "a"
			`,
			`bar=b`,
			&struct {
				Foo string `hcl:"foo"`
			}{
				Foo: "a",
			},
			`Unexpected argument "bar".`,
		},
		"override attribute in existing unlabelled block": {
			`
			block { foo = "a" }
			`,
			`block.foo=b`,
			&struct {
				Block *BlockNoLabels `hcl:"block,block"`
			}{
				Block: &BlockNoLabels{
					Foo: "b",
				},
			},
			``,
		},
		"new attribute in existing unlabelled block": {
			`
			block {}
			`,
			`block.foo=b`,
			&struct {
				Block *BlockNoLabels `hcl:"block,block"`
			}{
				Block: &BlockNoLabels{
					Foo: "b",
				},
			},
			``,
		},
		"override attribute in existing block with one label": {
			`
			block "a" { foo = "a" }
			block "b" { foo = "b" }
			`,
			`block.b.foo=c`,
			&struct {
				Block []BlockOneLabel `hcl:"block,block"`
			}{
				Block: []BlockOneLabel{
					{Name: "a", Foo: "a"},
					{Name: "b", Foo: "c"},
				},
			},
			``,
		},
		"create new block with one label": {
			`
			block "a" { foo = "a" }
			`,
			`block.b.foo=b`,
			&struct {
				Block []BlockOneLabel `hcl:"block,block"`
			}{
				Block: []BlockOneLabel{
					{Name: "a", Foo: "a"},
					{Name: "b", Foo: "b"},
				},
			},
			``,
		},
		"override attribute in existing block with two labels": {
			`
			block "foo" "a" { foo = "a" }
			block "foo" "b" { foo = "b" }
			`,
			`block.foo.b.foo=c`,
			&struct {
				Block []BlockTwoLabels `hcl:"block,block"`
			}{
				Block: []BlockTwoLabels{
					{Type: "foo", Name: "a", Foo: "a"},
					{Type: "foo", Name: "b", Foo: "c"},
				},
			},
			``,
		},
		"create new block with two labels": {
			`
			block "foo" "a" { foo = "a" }
			`,
			`block.foo.b.foo=b`,
			&struct {
				Block []BlockTwoLabels `hcl:"block,block"`
			}{
				Block: []BlockTwoLabels{
					{Type: "foo", Name: "a", Foo: "a"},
					{Type: "foo", Name: "b", Foo: "b"},
				},
			},
			``,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			f, diags := hclsyntax.ParseConfig([]byte(test.Config), "", hcl.Pos{})
			if diags.HasErrors() {
				t.Fatalf("config has problems: %s", diags.Error())
			}

			o, diags := ParseCLIArgument(test.Arg)
			if diags.HasErrors() {
				t.Fatalf("arg has problems: %s", diags.Error())
			}

			body := ApplyOverlays(f.Body, o)

			wantType := reflect.TypeOf(test.Want).Elem()
			got := reflect.New(wantType).Interface() // zero value of same type as "want"
			diags = gohcl.DecodeBody(body, nil, got)
			if diags.HasErrors() {
				errStr := diags.Error()
				if test.WantErr == "" {
					t.Fatalf("unexpected problems: %s", errStr)
				}
				if !strings.Contains(errStr, test.WantErr) {
					t.Fatalf("wrong error\ngot: %s\nshould contain: %s", test.WantErr, errStr)
				}
				return
			}
			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Fatalf("incorrect result\n%s", diff)
			}
		})
	}
}
