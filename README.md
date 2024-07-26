# rpath

```
❯ ./dist/rpath --help
rpath - find path of the element present at specified position

Usage:

  rpath [flags] CATEGORY [FILE]

Available CATEGORY:
- yaml, yml
- json

Flags:
  -column int
        Column number of target, 1-based
  -debug
        Enable debug logs
  -line int
        Line number of target, 1-based
  -offset int
        Offset of target, 0-based (default -1)
  -verbose
        Verbose output
```

# Examples

``` sh
❯ cat - <<EOS | rpath -line 6 -column 10 yaml
apiVersion: v1
kind: Text
metadata:
  name: sometext
spec:
  text1: テキスト
  text2: text
EOS
$.spec.text1
```

``` sh
❯ cat - <<EOS | rpath -line 8 -column 14 json
{
  "apiVersion": "v1",
  "kind": "Text",
  "metadata": {
    "name": "sometext"
  },
  "spec": {
    "text1": "テキスト",
    "text2": "text"
  }
}
EOS
.["spec"]["text1"]
```

# Tasks

## test

Run unit tests and e2e tests.

``` sh
go test -v -cover -race ./...
```

## build

Build executable binary to `dist/rpath`.

``` sh
go build -trimpath -v -o dist/rpath ./cmd/rpath
```

## lint

Run linters.

Requires: vet, vuln

## vet

Examine code.

``` sh
go vet ./...
```

## vuln

Find vulnerabilities.

``` sh
go run golang.org/x/vuln/cmd/govulncheck ./...
```
