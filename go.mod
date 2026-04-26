module github.com/tommymorgan/tsgolint

go 1.26

require github.com/microsoft/typescript-go v0.0.0-00010101000000-000000000000

// The fork lives as a sibling checkout. Releases will pin a commit/tag.
replace github.com/microsoft/typescript-go => ../typescript-go
