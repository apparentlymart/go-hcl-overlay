// Package hcloverlay is an extension to HCL for modelling "overlays",
// which are objects that can contribute new arguments and blocks or
// add new arguments and blocks to an existing HCL body.
//
// The ability to apply overlays implies some additional constraints on the
// design of the underlying language in comparison to plain use of HCL, but
// it can be useful in situations where e.g. HCL is bein used for configuration
// of a system but it's also desirable to be able to override particular
// configuration elements via command line arguments.
//
// The specific additional constraints implied by an overlay depend on the
// particular overlay implementation. See the documentation of the overlay
// factory functions in this package for more details.
package hcloverlay
