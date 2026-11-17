# ixgowrappers

Generated symbol tables that make GoBlog's packages and selected third-party
packages available to plugins running in the [iXGo](https://github.com/goplus/ixgo)
interpreter.

The files in the subdirectories are generated with `qexp`. Regenerate them with:

```bash
cd pkgs/ixgowrappers
go generate ./...
```

`wrappers.go` also blank-imports `github.com/goplus/ixgo/pkg`, which registers
the Go standard library.
