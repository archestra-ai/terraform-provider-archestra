package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// TestExamplesCoverage fails when a registered resource or data source lacks
// its `examples/{resources|data-sources}/<type>/<resource|data-source>.tf`
// file. The example is the user-facing how-to — tfplugindocs renders it
// inline into `docs/`, and the schema reference alone doesn't show how
// arguments compose. A missing example forces users to read the source.
func TestExamplesCoverage(t *testing.T) {
	t.Parallel()

	prov := New("test")()
	ctx := t.Context()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("locate repo root: %s", err)
	}

	t.Run("resources", func(t *testing.T) {
		for _, ctor := range prov.Resources(ctx) {
			r := ctor()
			var meta resource.MetadataResponse
			r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "archestra"}, &meta)
			dir := filepath.Join(repoRoot, "examples", "resources", meta.TypeName)

			tfPath := filepath.Join(dir, "resource.tf")
			if _, err := os.Stat(tfPath); err != nil {
				t.Errorf("missing example for %s — expected %s", meta.TypeName, tfPath)
			}

			// Resources that support import must ship an import.sh so
			// tfplugindocs renders the "Import" section in the resource doc.
			if _, ok := r.(resource.ResourceWithImportState); ok {
				importPath := filepath.Join(dir, "import.sh")
				if _, err := os.Stat(importPath); err != nil {
					t.Errorf("missing import.sh for %s — expected %s", meta.TypeName, importPath)
				}
			}
		}
	})

	t.Run("data_sources", func(t *testing.T) {
		for _, ctor := range prov.DataSources(ctx) {
			d := ctor()
			var meta datasource.MetadataResponse
			d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "archestra"}, &meta)
			path := filepath.Join(repoRoot, "examples", "data-sources", meta.TypeName, "data-source.tf")
			if _, err := os.Stat(path); err != nil {
				t.Errorf("missing example for %s — expected %s", meta.TypeName, path)
			}
		}
	})
}
