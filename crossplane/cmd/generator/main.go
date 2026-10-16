package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/crossplane/upjet/v2/pkg/pipeline"

	"github.com/archestra-ai/provider-archestra/config"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--references" {
		if err := config.PreserveReferenceCollectionOmission(os.Args[2]); err != nil {
			panic(err)
		}
		return
	}
	if len(os.Args) < 2 || os.Args[1] == "" {
		panic("root directory is required to be given as argument")
	}
	rootDir := os.Args[1]
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		panic(fmt.Sprintf("cannot calculate the absolute path with %s", rootDir))
	}
	cluster, namespaced := config.GetProvider(), config.GetProviderNamespaced()
	pipeline.Run(cluster, namespaced, absRootDir)
	if err := config.PreserveRequiredCollections(absRootDir, cluster, namespaced); err != nil {
		panic(err)
	}
}
