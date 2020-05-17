# HCL "Overlays"

This library is intended to be used by applications that use configuration
languages built with [HCL](https://hcl.readthedocs.io/), adding some additional
utilities for applying "surgical" modifications to the resulting configuration
by mechanisms like command line arguments.

For example, [CLI options overlays](./examples/cliargs) allow using the same
HCL content schema as for a main configuration language to decode command line
options that override specific arguments in the configuration.

For more information, see
[the package documentation](https://pkg.go.dev/github.com/apparentlymart/go-hcl-overlay/hcloverlay).
