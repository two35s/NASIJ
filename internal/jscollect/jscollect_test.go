package jscollect

import (
	"testing"
)

func TestScan_Empty(t *testing.T) {
	s := New()
	r := s.Scan("", "")
	if r == nil {
		t.Fatal("expected non-nil result")
	}
	if len(r.ScriptURLs) != 0 {
		t.Errorf("expected 0 script URLs, got %d", len(r.ScriptURLs))
	}
	if len(r.DynamicImports) != 0 {
		t.Errorf("expected 0 dynamic imports, got %d", len(r.DynamicImports))
	}
	if len(r.SourceMaps) != 0 {
		t.Errorf("expected 0 source maps, got %d", len(r.SourceMaps))
	}
}

func TestScan_ScriptURLs(t *testing.T) {
	src := `
		var api = "https://api.example.com/v1";
		var img = "/assets/logo.png";
		var cdn = "//cdn.example.com/lib.js";
		var path = "./utils/helper.js";
		var plain = "not-a-url";
	`
	s := New()
	r := s.Scan(src, "https://example.com/page.js")
	if len(r.ScriptURLs) != 4 {
		t.Fatalf("expected 4 script URLs, got %d: %v", len(r.ScriptURLs), r.ScriptURLs)
	}
}

func TestScan_DynamicImports(t *testing.T) {
	src := `
		import("./lazy.js");
		const mod = import("./pages/Home.js");
		import("https://cdn.example.com/module.js");
	`
	s := New()
	r := s.Scan(src, "https://example.com/main.js")
	if len(r.DynamicImports) != 3 {
		t.Fatalf("expected 3 dynamic imports, got %d: %v", len(r.DynamicImports), r.DynamicImports)
	}
}

func TestScan_DynamicImports_Dedup(t *testing.T) {
	src := `
		import("./lazy.js");
		import("./lazy.js");
	`
	s := New()
	r := s.Scan(src, "https://example.com/main.js")
	if len(r.DynamicImports) != 1 {
		t.Fatalf("expected 1 dynamic import (dedup), got %d: %v", len(r.DynamicImports), r.DynamicImports)
	}
}

func TestScan_WebpackPublicPath(t *testing.T) {
	src := `__webpack_public_path__ = "/dist/";`
	s := New()
	r := s.Scan(src, "https://example.com/app.js")
	if len(r.WebpackChunks) != 1 || r.WebpackChunks[0] != "/dist/" {
		t.Fatalf("expected webpack public path, got %v", r.WebpackChunks)
	}
}

func TestScan_WebpackChunks(t *testing.T) {
	src := `
		var chunk = "chunk-a1b2c3d4";
		__webpack_chunk_load__("vendor~main.chunk.js");
	`
	s := New()
	r := s.Scan(src, "https://example.com/app.js")
	if len(r.WebpackChunks) < 2 {
		t.Fatalf("expected at least 2 webpack chunks, got %d: %v", len(r.WebpackChunks), r.WebpackChunks)
	}
}

func TestScan_ViteAssets(t *testing.T) {
	src := `
		const url = new URL('./assets/logo.svg', import.meta.url);
		const modules = import.meta.glob('./pages/**/*.js');
	`
	s := New()
	r := s.Scan(src, "https://example.com/main.js")
	if len(r.ViteAssets) != 2 {
		t.Fatalf("expected 2 vite assets, got %d: %v", len(r.ViteAssets), r.ViteAssets)
	}
}

func TestScan_SourceMaps(t *testing.T) {
	src := `
		console.log("hello");
		//# sourceMappingURL=main.js.map
	`
	s := New()
	r := s.Scan(src, "https://example.com/main.js")
	if len(r.SourceMaps) != 1 {
		t.Fatalf("expected 1 source map, got %d", len(r.SourceMaps))
	}
	if r.SourceMaps[0].URL != "main.js.map" {
		t.Errorf("expected main.js.map, got %s", r.SourceMaps[0].URL)
	}
}

func TestScan_SourceMapsBlock(t *testing.T) {
	src := `/*# sourceMappingURL=app.js.map */`
	s := New()
	r := s.Scan(src, "https://example.com/app.min.js")
	if len(r.SourceMaps) != 1 {
		t.Fatalf("expected 1 source map, got %d", len(r.SourceMaps))
	}
}

func TestScan_InlineSourceMap(t *testing.T) {
	src := `//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozfQ==`
	s := New()
	r := s.Scan(src, "https://example.com/app.js")
	if len(r.SourceMaps) != 1 {
		t.Fatalf("expected 1 source map, got %d", len(r.SourceMaps))
	}
	if !r.SourceMaps[0].Inline {
		t.Error("expected inline source map")
	}
}

func TestScan_AllPatterns(t *testing.T) {
	src := `
		// External URL
		var api = "https://api.example.com/data";

		// Dynamic imports
		import("./lazy.js");

		// Webpack public path
		__webpack_public_path__ = "/dist/";
		var chunk = "chunk-a1b2c3";

		// Vite
		const url = new URL('./assets/icon.svg', import.meta.url);
		const mods = import.meta.glob('./pages/*.js');

		// Source map
		//# sourceMappingURL=bundle.js.map
	`
	s := New()
	r := s.Scan(src, "https://example.com/main.js")

	if len(r.ScriptURLs) == 0 {
		t.Error("expected script URLs")
	}
	if len(r.DynamicImports) == 0 {
		t.Error("expected dynamic imports")
	}
	if len(r.WebpackChunks) == 0 {
		t.Error("expected webpack chunks")
	}
	if len(r.ViteAssets) == 0 {
		t.Error("expected vite assets")
	}
	if len(r.SourceMaps) == 0 {
		t.Error("expected source maps")
	}
}
