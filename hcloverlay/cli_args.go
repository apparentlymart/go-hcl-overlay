package hcloverlay

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// ParseCLIArgument expects a string consisting of a sequence of dot-separated
// identifiers, followed by an equals sign "=" and then a sequence of
// arbitrary characters.
//
// The part before the equals sign is interpreted as a sequence of traversals
// through the configuration to an argument to set or override. The part
// after the equals sign is a string value to set the argument to.
//
// The result is an overlay that replaces the value of the indicated argument
// with the given string value.
//
// This overlay is intended to be used with HCL-based configuration languages
// that have the following constraints in addition to those of the HCL infoset:
//
//     - All blocks must be uniquely identified by their block type and labels.
//       If multiple blocks appear in the same body with the same header,
//       an override for that header will apply only to the first such block
//       in the source configuration.
//
//     - All argument names, block types, and block labels must be valid HCL
//       identifiers, as decided by hclsyntax.ValidIdentifier .
//
//     - All arguments that may be overridden must accept strings, either
//       directly or as the input to a type conversion.
//
// If the given string traverses through a block whose type is derived by the
// schema but that does not exist in the configuration being overridden then
// the overlay will create a new block with the appropriate labels that
// contains only the specified argument.
//
// Argument values overridden by CLI argument overlays will have no source
// location information, so an application using overlays returned from this
// method must be prepared to accept zero-value hcl.Range values and treat
// them as the absense of a range if accessing the ranges associated with
// attributes and blocks in resulting content.
func ParseCLIArgument(raw string) (Overlay, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	eq := strings.IndexByte(raw, '=')
	if eq < 1 { // if the equals is missing or if it's at the start of the string
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid argument",
			Detail:   fmt.Sprintf("Invalid argument %q: must be a configuration setting, followed by an equals sign, and then a value for that setting.", raw),
		})
		return nil, diags
	}
	path, val := raw[:eq], raw[eq+1:]

	steps := strings.Split(path, ".")
	for _, step := range steps {
		if !hclsyntax.ValidIdentifier(step) {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid argument",
				Detail:   fmt.Sprintf("Invalid component %q in argument %q: dot-separated parts must be a letter followed by zero or more letters, digits, or underscores.", step, path),
			})
		}
	}
	if diags.HasErrors() {
		return nil, diags
	}

	return &cliArgOverlay{
		fullPath: path,
		steps:    steps,
		val:      val,
	}, nil
}

// ExtractCLIOptions interprets the given slice as a sequence of command
// line arguments and identifies any that have the conventional "--" prefix
// for named optional arguments followed by identifiers that correspond to
// attributes or block types in the given schema and attempts to produce an
// overlay for each one using the behaviors described for ParseCLIArgument.
//
// Additionally, if one of the arguments is literally "--" then
// ExtractCLIOptions will not interpret any subsequent arguments as overlays.
//
// ExtractCLIOptions returns a sequence of overlays and a new slice of strings
// that contains all of the arguments from the given slice that were not
// interpreted as overlays, so that they might be used for further command
// line processing.
func ExtractCLIOptions(args []string, schema *hcl.BodySchema) ([]Overlay, []string, hcl.Diagnostics) {
	var remain []string
	var overlays []Overlay
	var diags hcl.Diagnostics

	for i, arg := range args {
		if arg == "--" {
			remain = append(remain, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "--") {
			remain = append(remain, arg)
			continue
		}
		raw := arg[2:] // trim "--"" prefix
		match := raw
		sep := strings.IndexAny(match, ".=")
		if sep != -1 {
			match = match[:sep]
		}
		matched := false
		for _, attrS := range schema.Attributes {
			if attrS.Name == match {
				matched = true
				break
			}
		}
		for _, blockS := range schema.Blocks {
			if blockS.Type == match {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		o, moreDiags := ParseCLIArgument(raw)
		diags = append(diags, moreDiags...)
		if o != nil {
			overlays = append(overlays, o)
		}
	}

	return overlays, args, diags
}

type cliArgOverlay struct {
	fullPath string // full path as originally given, for use in error messages
	steps    []string
	val      string
}

func (o *cliArgOverlay) ApplyOverlay(content *hcl.BodyContent, schema *hcl.BodySchema) (*hcl.BodyContent, hcl.Diagnostics) {
	ret, remain, diags := o.PartialApplyOverlay(content, schema)
	if remain != nil {
		// If we have a "remain" then our path wasn't accepted by the
		// schema, which means the argument was invalid.
		diags = diags.Append(o.invalidArgError())
	}
	return ret, diags
}

func (o *cliArgOverlay) PartialApplyOverlay(content *hcl.BodyContent, schema *hcl.BodySchema) (*hcl.BodyContent, Overlay, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	// There should be either an attribute or block type in the given
	// schema that matches our first step. That'll tell us how to interpret
	// the remainder of the steps (if any).
	name := o.steps[0]

	for _, attrS := range schema.Attributes {
		if attrS.Name != name {
			continue
		}
		if len(o.steps) != 1 {
			diags = diags.Append(o.invalidArgError())
			return content, nil, diags
		}

		// If we get here then we're overriding the attribute described by attrS
		content.Attributes[name] = &hcl.Attribute{
			Name: o.steps[0],
			Expr: hcl.StaticExpr(cty.StringVal(o.val), hcl.Range{}),
		}
		return content, nil, diags
	}

	for _, blockS := range schema.Blocks {
		if blockS.Type != name {
			continue
		}
		// We must have at least enough subsequent steps for all of the
		// labels this block type expects and at least one additional to
		// continue traversing inside the selected block.
		needStepCount := 1 + 1 + len(blockS.LabelNames)
		if len(o.steps) < needStepCount {
			diags = diags.Append(o.invalidArgError())
			return content, nil, diags
		}

		// If we get here then we need to hunt in content.Blocks for the
		// first block that has the selected type and labels, and we'll
		// then apply the remaining steps in our path as an overlay on its
		// body.
		wantLabels := o.steps[1 : len(blockS.LabelNames)+1]
		remainingSteps := o.steps[len(wantLabels)+1:]
		subOverlay := o.subOverlay(remainingSteps)
		for _, block := range content.Blocks {
			if block.Type != blockS.Type {
				continue
			}
			if !o.labelsMatch(block.Labels, wantLabels) {
				continue
			}
			// We've found it!
			block.Body = ApplyOverlays(block.Body, subOverlay)
			return content, nil, diags
		}

		// If we get here then we didn't find a suitable block to override,
		// and so we'll construct ourselves a new one. Its body will
		// essentially be just the effect of our overlay, which we'll achieve
		// by applying it to an empty body.
		block := &hcl.Block{
			Type:        blockS.Type,
			Body:        ApplyOverlays(hcl.EmptyBody(), subOverlay),
			Labels:      wantLabels,
			LabelRanges: make([]hcl.Range, len(wantLabels)), // must have same length as Labels even though it's all zero values
		}
		content.Blocks = append(content.Blocks, block)
		return content, nil, diags
	}

	// If we fall out here then this overlay doesn't produce something
	// the schema calls for, so we'll just do nothing and return ourselves
	// again for a later attempt.
	return content, o, diags
}

func (o *cliArgOverlay) ApplyJustAttributes(attrs hcl.Attributes) (hcl.Attributes, hcl.Diagnostics) {
	if len(o.steps) != 1 {
		// In "just attributes" mode, we must have only a single step because
		// there can be no blocks for us to traverse through.
		var diags hcl.Diagnostics
		diags = diags.Append(o.invalidArgError())
		return attrs, diags
	}

	attrs[o.steps[0]] = &hcl.Attribute{
		Name: o.steps[0],
		Expr: hcl.StaticExpr(cty.StringVal(o.val), hcl.Range{}),
	}

	return attrs, nil
}

func (o *cliArgOverlay) subOverlay(remainingSteps []string) *cliArgOverlay {
	return &cliArgOverlay{
		fullPath: o.fullPath,
		val:      o.val,
		steps:    remainingSteps,
	}
}

func (o *cliArgOverlay) labelsMatch(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func (o *cliArgOverlay) invalidArgError() *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid argument",
		Detail:   fmt.Sprintf("Unexpected argument %q.", o.fullPath),
	}
}
