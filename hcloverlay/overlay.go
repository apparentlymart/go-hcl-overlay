package hcloverlay

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

// An Overlay is an object that can be applied to a body using the OverlayBody
// function, in which case it will get an opportunity to modify the result of
// decoding that body, usually by adding or replacing attributes or blocks.
type Overlay interface {
	// ApplyOverlay receives the result of decoding a body along with the
	// schema that was used to decode that body and produces a new body
	// that incorporates whatever additional or replacement content the
	// overlay represents.
	//
	// ApplyOverlay returns an error if the given schema does not call for
	// any of the content changes that the overlay represents. These errors
	// will usually be phrased from the perspective that the overlay itself
	// is invalid, under the assumption that the overlay is being applied
	// at the request of an end-user.
	//
	// Although conceptually this method consumes a content object and produces
	// a replacement object, in practice implementations are free to modify
	// the given object directly and return that same object in the common
	// case where the overlay changes are surgical and it would be wasteful
	// to allocate an entirely new copy.
	ApplyOverlay(content *hcl.BodyContent, schema *hcl.BodySchema) (*hcl.BodyContent, hcl.Diagnostics)

	// PartialApplyOverlay is a variant of ApplyOverlay that should apply
	// as much of its additional or replacement content as possible but
	// should, if the schema does not call for any part of that content,
	// return a new Overlay that would apply the remaining changes.
	//
	// PartialApplyOverlay will return a nil new Overlay in the case where it
	// has applied all of the changes it represents.
	PartialApplyOverlay(content *hcl.BodyContent, schema *hcl.BodySchema) (*hcl.BodyContent, Overlay, hcl.Diagnostics)

	// ApplyJustAttributes applies all of the attribute modifications
	// represented by the overlay, returning an error if any of the
	// modifications call for the addition of a block.
	//
	// Conceptually, ApplyJustAttributes constructs a synthetic schema
	// containing whatever attributes the overlay implies and no block types,
	// and then calls ApplyOverlay with that synthetic schema to overlay
	// all of the implied attributes. (Actual implementation may vary, naturally.)
	ApplyJustAttributes(attrs hcl.Attributes) (hcl.Attributes, hcl.Diagnostics)
}

// ApplyOverlays wraps the given HCL body such that when calling the various
// decoding methods the result will incorporate the result of applying the
// given overlays.
//
// If multiple overlays are given, they will be applied in the given order
// with each subsequent overlay consuming the result of the one preceding
// it.
//
// When applying overlays to a body, the original body is required to be
// valid per the schema except that the "Required" flag for attributes is
// not enforced. Requiredness is instead enforced on the result of applying
// the overlays.
func ApplyOverlays(body hcl.Body, overlays ...Overlay) hcl.Body {
	if len(overlays) == 0 {
		return body // wrapping is pointless
	}
	return &applyBody{
		inner:    body,
		overlays: overlays,
	}
}

type applyBody struct {
	inner    hcl.Body
	overlays []Overlay
}

func (b *applyBody) Content(schema *hcl.BodySchema) (*hcl.BodyContent, hcl.Diagnostics) {
	// modSchema is the same as schema except that attributes are
	// always optional. This allows is to delay enforcing requiredness
	// until overlaying is complete.
	modSchema := b.schemaNoRequired(schema)

	content, diags := b.inner.Content(modSchema)
	for _, ov := range b.overlays {
		var moreDiags hcl.Diagnostics
		content, moreDiags = ov.ApplyOverlay(content, modSchema)
		diags = append(diags, moreDiags...)
	}

	return b.prepareContent(content, schema, diags)
}

func (b *applyBody) PartialContent(schema *hcl.BodySchema) (*hcl.BodyContent, hcl.Body, hcl.Diagnostics) {
	// modSchema is the same as schema except that attributes are
	// always optional. This allows is to delay enforcing requiredness
	// until overlaying is complete.
	modSchema := b.schemaNoRequired(schema)

	content, remain, diags := b.inner.PartialContent(modSchema)
	var remainOverlays []Overlay
	for _, ov := range b.overlays {
		var moreDiags hcl.Diagnostics
		var remainOverlay Overlay
		content, remainOverlay, moreDiags = ov.PartialApplyOverlay(content, modSchema)
		diags = append(diags, moreDiags...)
		if remainOverlay != nil {
			remainOverlays = append(remainOverlays, remainOverlay)
		}
	}

	remain = ApplyOverlays(remain, remainOverlays...)

	content, diags = b.prepareContent(content, schema, diags)
	return content, remain, diags
}

func (b *applyBody) JustAttributes() (hcl.Attributes, hcl.Diagnostics) {
	attrs, diags := b.inner.JustAttributes()
	for _, ov := range b.overlays {
		var moreDiags hcl.Diagnostics
		attrs, moreDiags = ov.ApplyJustAttributes(attrs)
		diags = append(diags, moreDiags...)
	}
	return attrs, diags
}

func (b *applyBody) MissingItemRange() hcl.Range {
	return b.inner.MissingItemRange()
}

func (b *applyBody) prepareContent(result *hcl.BodyContent, schema *hcl.BodySchema, diags hcl.Diagnostics) (*hcl.BodyContent, hcl.Diagnostics) {

	for _, attrS := range schema.Attributes {
		if !attrS.Required {
			continue
		}
		if _, exists := result.Attributes[attrS.Name]; !exists {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Missing required argument",
				Detail:   fmt.Sprintf("The argument %q is required, but no definition was found.", attrS.Name),
				Subject:  b.MissingItemRange().Ptr(),
			})
		}
	}

	return result, diags
}

func (b *applyBody) schemaNoRequired(given *hcl.BodySchema) *hcl.BodySchema {
	ret := &hcl.BodySchema{
		Blocks: given.Blocks,
	}

	if len(given.Attributes) != 0 {
		ret.Attributes = make([]hcl.AttributeSchema, len(given.Attributes))
		copy(ret.Attributes, given.Attributes)
		for i := range ret.Attributes {
			ret.Attributes[i].Required = false
		}
	}

	return ret
}
