package handler

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltInChartArchivesContainRequiredTemplateFiles(t *testing.T) {
	archives, err := filepath.Glob(filepath.Join("..", "..", "data", "charts", "*.tar.gz"))
	if err != nil {
		t.Fatalf("list built-in chart archives: %v", err)
	}
	if len(archives) == 0 {
		t.Fatalf("expected built-in chart archives under data/charts")
	}

	for _, archivePath := range archives {
		t.Run(filepath.Base(archivePath), func(t *testing.T) {
			required := map[string]bool{
				"chart/":                 false,
				"platform-manifest.yaml": false,
				"preset-values.yaml":     false,
			}

			file, err := os.Open(archivePath)
			if err != nil {
				t.Fatalf("open archive: %v", err)
			}
			defer file.Close()

			gz, err := gzip.NewReader(file)
			if err != nil {
				t.Fatalf("read gzip: %v", err)
			}
			defer gz.Close()

			reader := tar.NewReader(gz)
			for {
				header, err := reader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("read tar entry: %v", err)
				}

				name := strings.TrimLeft(filepath.ToSlash(header.Name), "./")
				switch {
				case name == "chart" || strings.HasPrefix(name, "chart/"):
					required["chart/"] = true
				case name == "platform-manifest.yaml":
					required["platform-manifest.yaml"] = true
				case name == "preset-values.yaml":
					required["preset-values.yaml"] = true
				}
			}

			for name, found := range required {
				if !found {
					t.Fatalf("archive missing %s", name)
				}
			}
		})
	}
}
