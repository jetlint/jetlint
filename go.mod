module github.com/jetlint/jetlint

go 1.26

require github.com/microsoft/typescript-go v0.2.3

require (
	github.com/go-json-experiment/json v0.0.0-20260214004413-d219187c3433 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
)

// Pinned to the jetlint fork because the wrapper API surface jetlint
// depends on hasn't landed in upstream microsoft/typescript-go yet.
// When the fork's commits are upstreamed, this directive can be removed
// and the require line can point at an upstream tag.
replace github.com/microsoft/typescript-go => github.com/jetlint/typescript-go v0.2.3
