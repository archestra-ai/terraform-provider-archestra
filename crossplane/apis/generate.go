//go:build generate
// +build generate

// NOTE: See the below link for details on what is happening here.
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

// Generated output is cleaned by the Makefile before go generate enumerates packages.

// Reuse resource descriptions and HCL examples; field docs come from the schema.
//go:generate go run github.com/crossplane/upjet/v2/cmd/scraper --name archestra-ai/archestra --resource-prefix archestra --field-xpath //ul/li/code[false()] --repo ../../docs/resources --out ../config/provider-metadata.yaml

// Run Upjet generator
//go:generate go run ../cmd/generator/main.go ..

// Generate deepcopy methodsets and CRD manifests
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=../hack/boilerplate.go.txt paths=./... crd:allowDangerousTypes=true,crdVersions=v1 output:artifacts:config=../package/crds

// Generate crossplane-runtime methodsets (resource.Claim, etc)
//go:generate go run -tags generate github.com/crossplane/crossplane-tools/cmd/angryjet generate-methodsets --header-file=../hack/boilerplate.go.txt ./...

// Preserve unmanaged reference collections after angryjet emits resolvers.
//go:generate go run ../cmd/generator/main.go --references ..

// Keep the package README in sync with the quick start.
//go:generate python3 ../hack/generate-readme.py

package apis

import (
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen" //nolint:typecheck

	_ "github.com/crossplane/crossplane-tools/cmd/angryjet" //nolint:typecheck
)
