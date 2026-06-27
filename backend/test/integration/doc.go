// Package integration holds the build-tagged end-to-end integration tests.
//
// The real test files carry a //go:build integration tag, so they are excluded
// from the default `go test ./...`. This untagged file keeps the package valid
// for the default build (otherwise the toolchain errors with "build constraints
// exclude all Go files"). Run the suite with: go test -tags=integration ./test/integration/...
package integration
