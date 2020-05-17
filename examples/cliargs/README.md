# CLI Arguments Example

This directory contains a small example of deriving configuration overlays
from command line options.

Switch into this directory (e.g. with `cd`) and then run the example using
`go run .` . With no arguments, it prints usage information:

```
go run . [options] <config-file>

Options:
  --json, -j
      Produce JSON output instead of human-oriented output.

  --io_mode=MODE
      Override the io_mode configuration argument.

  --service.TYPE.NAME.listen_addr=ADDR
	  Override the listen address for the service with the given TYPE and NAME.
```

The `<config-file>` is an HCL file following a schema similar to the one used
in [the `gohcl` section of the HCL manual](https://hcl.readthedocs.io/en/latest/go_decoding_gohcl.html). The file [`config.hcl`](./config.hcl) in this directory
shows an example of the expected format, and you can pass this configuration
file to the example program:

```
$ go run . config.hcl 
The IO mode is "async"

- Service "http" "web_proxy":
  The listen address is 127.0.0.1:8080
```

The example program accepts a mixture of "normal" command line options,
processed using the "pflag" package and configuration-overriding options
processed using `hcloverlay`'s CLI option syntax.

The `--json` option is a normal flag and switches the program to produce
JSON output:

```
$ go run . config.hcl --json
{
  "io_mode": "async",
  "services": [
    {
      "type": "http",
      "name": "web_proxy",
      "listen_addr": "127.0.0.1:8080"
    }
  ]
}
```

The other two command line option types correspond with arguments in the
configuration language. An `--io-mode=...` option simply overrides the
top-level `io_mode` option:

```
$ go run . config.hcl --io_mode=blocking
The IO mode is "blocking"

- Service "http" "web_proxy":
  The listen address is 127.0.0.1:8080
```

Options starting with `--service` allow overriding arguments inside
a `service` block defined in the configuration, matching based on the
type and name labels on the blocks:

```
$ go run . config.hcl --service.http.web_proxy.listen_addr=127.0.0.1:2000
The IO mode is "async"

- Service "http" "web_proxy":
  The listen address is 127.0.0.1:2000
```

The override will be applied to the first `service` block with a matching
type and name. The CLI override syntax is intended to be used with HCL-based
languages where blocks are uniquely identified by their headers, although that
is not enforced by this example.

If there is no `service` block with the given labels, the option creates an
entirely new block with those labels:

```
$ go run . config.hcl --service.gopher.proxy.listen_addr=127.0.0.1:2000
The IO mode is "async"

- Service "http" "web_proxy":
  The listen address is 127.0.0.1:8080

- Service "gopher" "proxy":
  The listen address is 127.0.0.1:2000
```

The `--io_mode` and `--service...` options are decoded subject to the same
schema as the configuration language itself, so attempting to set a `service`
argument that isn't part of the schema will produce an error:

```
$ go run . config.hcl --service.http.web_proxy.invalid=foo
Error: Invalid argument

Unexpected argument "service.http.web_proxy.invalid".
```
