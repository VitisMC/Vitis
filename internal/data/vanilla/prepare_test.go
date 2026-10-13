package vanilla

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// runtimeFixture builds a minimal resource bundle with matching version metadata.
func runtimeFixture(t *testing.T, mutate func(map[string][]byte)) (Version, []byte) {
	t.Helper()
	var expected Version
	metadata := []byte(`{"id":"1.21.4","world_version":4189,"protocol_version":769,"pack_version":{"data":61}}`)
	if err := json.Unmarshal(metadata, &expected); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"version.json": metadata}
	for _, directory := range runtimeDirectories {
		files["data/minecraft/"+directory+"/test.json"] = []byte(`{}`)
	}
	if mutate != nil {
		mutate(files)
	}
	inner := zipBytes(t, files)
	return expected, zipBytes(t, map[string][]byte{"META-INF/versions/1.21.4/server-1.21.4.jar": inner})
}

// serveBundle exposes Mojang-shaped metadata and a JAR over local HTTPS.
func serveBundle(t *testing.T, jar []byte, mutate func(string, map[string]any)) *httptest.Server {
	t.Helper()
	hash := sha1.Sum(jar)
	server := httptest.NewUnstartedServer(nil)
	server.Config.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var value map[string]any
		switch request.URL.Path {
		case "/manifest":
			value = map[string]any{"versions": []any{map[string]any{"id": "1.21.4", "url": server.URL + "/version"}}}
		case "/version":
			value = map[string]any{"id": "1.21.4", "downloads": map[string]any{"server": map[string]any{
				"url": server.URL + "/jar", "sha1": hex.EncodeToString(hash[:]), "size": len(jar),
			}}}
		case "/jar":
			if _, err := writer.Write(jar); err != nil {
				t.Logf("write fixture: %v", err)
			}
			return
		default:
			http.NotFound(writer, request)
			return
		}
		if mutate != nil {
			mutate(request.URL.Path, value)
		}
		if err := json.NewEncoder(writer).Encode(value); err != nil {
			t.Logf("write metadata: %v", err)
		}
	})
	server.StartTLS()
	t.Cleanup(server.Close)
	server.Client().CheckRedirect = secureRedirect
	return server
}

// resourceRoot resolves platform temporary-directory aliases before symlink checks.
func resourceRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// assertNoPublishedPack checks that failed preparation leaves no partial target or stage.
func assertNoPublishedPack(t *testing.T, output string) {
	t.Helper()
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial target: %v", err)
	}
	stages, err := filepath.Glob(filepath.Join(filepath.Dir(output), ".vanilla-stage-*"))
	if err != nil || len(stages) != 0 {
		t.Fatalf("stages remain: %v, %v", stages, err)
	}
}

// TestPrepareInstallsAndReuses verifies Java-free installation and offline reuse.
func TestPrepareInstallsAndReuses(t *testing.T) {
	expected, jar := runtimeFixture(t, nil)
	server := serveBundle(t, jar, nil)
	output := filepath.Join(resourceRoot(t), "version", "vanilla")
	if err := prepare(context.Background(), expected, output, server.Client(), server.URL+"/manifest"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "reports")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("runtime installation must not generate reports")
	}
	resource := filepath.Join(output, "data", "minecraft", "recipe", "test.json")
	before, err := os.ReadFile(resource)
	if err != nil {
		t.Fatal(err)
	}
	server.Close()
	if err := prepare(context.Background(), expected, output, server.Client(), server.URL+"/manifest"); err != nil {
		t.Fatalf("offline reuse: %v", err)
	}
	after, err := os.ReadFile(resource)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("existing resource changed: %v", err)
	}
}

// TestPrepareRejectsBadDownloads covers invalid metadata, content and version identity.
func TestPrepareRejectsBadDownloads(t *testing.T) {
	cases := []struct {
		name     string
		files    func(map[string][]byte)
		metadata func(string, map[string]any)
		want     string
	}{
		{name: "absent version", metadata: func(endpoint string, value map[string]any) {
			if endpoint == "/manifest" {
				value["versions"] = []any{}
			}
		}, want: "absent"},
		{name: "wrong metadata version", metadata: func(endpoint string, value map[string]any) {
			if endpoint == "/version" {
				value["id"] = "1.21.5"
			}
		}, want: "metadata"},
		{name: "checksum", metadata: func(endpoint string, value map[string]any) {
			if endpoint == "/version" {
				value["downloads"].(map[string]any)["server"].(map[string]any)["sha1"] = strings.Repeat("0", 40)
			}
		}, want: "checksum mismatch"},
		{name: "size", metadata: func(endpoint string, value map[string]any) {
			if endpoint == "/version" {
				server := value["downloads"].(map[string]any)["server"].(map[string]any)
				server["size"] = server["size"].(int) + 1
			}
		}, want: "size"},
		{name: "oversized download", metadata: func(endpoint string, value map[string]any) {
			if endpoint == "/version" {
				value["downloads"].(map[string]any)["server"].(map[string]any)["size"] = maxServerSize + 1
			}
		}, want: "metadata"},
		{name: "HTTP error", metadata: func(endpoint string, value map[string]any) {
			if endpoint == "/manifest" {
				value["versions"].([]any)[0].(map[string]any)["url"] = value["versions"].([]any)[0].(map[string]any)["url"].(string) + "-missing"
			}
		}, want: "HTTP 404"},
		{name: "insecure URL", metadata: func(endpoint string, value map[string]any) {
			if endpoint == "/version" {
				value["downloads"].(map[string]any)["server"].(map[string]any)["url"] = "http://example.com/server.jar"
			}
		}, want: "HTTPS"},
		{name: "wrong JAR ID", files: func(files map[string][]byte) {
			files["version.json"] = []byte(`{"id":"1.21.5","world_version":4189,"protocol_version":769,"pack_version":{"data":61}}`)
		}, want: "version mismatch"},
		{name: "wrong protocol", files: func(files map[string][]byte) {
			files["version.json"] = []byte(`{"id":"1.21.4","world_version":4189,"protocol_version":768,"pack_version":{"data":61}}`)
		}, want: "version mismatch"},
		{name: "wrong DataVersion", files: func(files map[string][]byte) {
			files["version.json"] = []byte(`{"id":"1.21.4","world_version":4188,"protocol_version":769,"pack_version":{"data":61}}`)
		}, want: "version mismatch"},
		{name: "wrong pack format", files: func(files map[string][]byte) {
			files["version.json"] = []byte(`{"id":"1.21.4","world_version":4189,"protocol_version":769,"pack_version":{"data":60}}`)
		}, want: "version mismatch"},
		{name: "missing category", files: func(files map[string][]byte) { delete(files, "data/minecraft/worldgen/test.json") }, want: "worldgen"},
		{name: "invalid JSON", files: func(files map[string][]byte) { files["data/minecraft/recipe/test.json"] = []byte(`{`) }, want: "invalid JSON"},
		{name: "traversal", files: func(files map[string][]byte) { files["data/../../escape"] = []byte("bad") }, want: "unsafe"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			expected, jar := runtimeFixture(t, test.files)
			server := serveBundle(t, jar, test.metadata)
			output := filepath.Join(resourceRoot(t), "vanilla")
			err := prepare(context.Background(), expected, output, server.Client(), server.URL+"/manifest")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v; want %q", err, test.want)
			}
			assertNoPublishedPack(t, output)
		})
	}
}

// TestPreparePreservesInvalidPack verifies that an existing damaged pack is never replaced.
func TestPreparePreservesInvalidPack(t *testing.T) {
	expected, _ := runtimeFixture(t, nil)
	output := filepath.Join(resourceRoot(t), "vanilla")
	if err := os.Mkdir(output, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(output, "keep")
	if err := os.WriteFile(marker, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	err := prepare(context.Background(), expected, output, nil, "unused")
	if err == nil || !strings.Contains(err.Error(), "preserved") {
		t.Fatalf("error = %v", err)
	}
	content, err := os.ReadFile(marker)
	if err != nil || string(content) != "original" {
		t.Fatalf("pack overwritten: %v", err)
	}
}

// TestPrepareCancellationAndRetry checks cleanup after an interrupted download.
func TestPrepareCancellationAndRetry(t *testing.T) {
	expected, jar := runtimeFixture(t, nil)
	server := serveBundle(t, jar, nil)
	original := server.Config.Handler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var interrupt atomic.Bool
	interrupt.Store(true)
	// The handler wrapper must be installed before serving requests.
	server.Config.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/jar" && interrupt.Load() {
			cancel()
			<-request.Context().Done()
			return
		}
		original.ServeHTTP(writer, request)
	})
	output := filepath.Join(resourceRoot(t), "vanilla")
	if err := prepare(ctx, expected, output, server.Client(), server.URL+"/manifest"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	assertNoPublishedPack(t, output)
	interrupt.Store(false)
	if err := prepare(context.Background(), expected, output, server.Client(), server.URL+"/manifest"); err != nil {
		t.Fatalf("retry: %v", err)
	}
}

// TestExtractRuntimeDataRejectsArchives checks unsafe entries and corrupt ZIP input.
func TestExtractRuntimeDataRejectsArchives(t *testing.T) {
	cases := []struct {
		name  string
		names []string
		mode  os.FileMode
	}{
		{"duplicate", []string{"data/test/a.json", "data/test/a.json"}, 0644},
		{"absolute", []string{"/data/test/a.json"}, 0644},
		{"backslash", []string{`data\test\a.json`}, 0644},
		{"drive", []string{"data/C:/a.json"}, 0644},
		{"case alias", []string{"data/test/a.json", "data/test/A.json"}, 0644},
		{"device", []string{"data/test/CON.json"}, 0644},
		{"trailing dot", []string{"data/test/a.json."}, 0644},
		{"symlink", []string{"data/test/link"}, os.ModeSymlink | 0777},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var buffer bytes.Buffer
			writer := zip.NewWriter(&buffer)
			for _, name := range test.names {
				header := &zip.FileHeader{Name: name}
				header.SetMode(test.mode)
				entry, err := writer.CreateHeader(header)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := fmt.Fprint(entry, `{}`); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := extractRuntimeData(context.Background(), buffer.Bytes(), filepath.Join(t.TempDir(), "pack")); err == nil {
				t.Fatal("accepted unsafe archive")
			}
		})
	}
	if err := extractRuntimeData(context.Background(), []byte("not a ZIP"), filepath.Join(t.TempDir(), "pack")); err == nil {
		t.Fatal("accepted corrupt ZIP")
	}
}

// TestPrepareCorruptBundle checks failed ZIP inspection after a verified download.
func TestPrepareCorruptBundle(t *testing.T) {
	expected, _ := runtimeFixture(t, nil)
	server := serveBundle(t, []byte("not a server ZIP"), nil)
	output := filepath.Join(resourceRoot(t), "vanilla")
	err := prepare(context.Background(), expected, output, server.Client(), server.URL+"/manifest")
	if err == nil || !strings.Contains(err.Error(), "open server JAR") {
		t.Fatalf("error = %v", err)
	}
	assertNoPublishedPack(t, output)
}

// TestArchiveLimitsAndIdentityEntries rejects oversized resources and duplicate metadata.
func TestArchiveLimitsAndIdentityEntries(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.CreateRaw(&zip.FileHeader{Name: "data/test/large.json", Method: zip.Store, UncompressedSize64: maxResourceSize + 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(nil); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractRuntimeData(context.Background(), buffer.Bytes(), filepath.Join(t.TempDir(), "pack")); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("error = %v", err)
	}
	file := &zip.File{FileHeader: zip.FileHeader{Name: "version.json"}}
	if _, err := uniqueZipEntry([]*zip.File{file, file}, "version.json"); err == nil {
		t.Fatal("accepted duplicate version metadata")
	}
}

// TestPreparationRejectsSymlinkParent checks that installation cannot write through aliases.
func TestPreparationRejectsSymlinkParent(t *testing.T) {
	root := resourceRoot(t)
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := makeResourceDirectory(filepath.Join(alias, "version")); err == nil {
		t.Fatal("followed symlink parent")
	}
}

// TestSecureRedirect rejects transport downgrades and redirect loops.
func TestSecureRedirect(t *testing.T) {
	for _, address := range []string{"http://example.com/jar", "https://user:password@example.com/jar"} {
		request, err := http.NewRequest(http.MethodGet, address, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := secureRedirect(request, nil); err == nil {
			t.Fatal("accepted unsafe redirect")
		}
	}
	request, err := http.NewRequest(http.MethodGet, "https://example.com/jar", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := secureRedirect(request, make([]*http.Request, 10)); err == nil {
		t.Fatal("accepted redirect loop")
	}
}

// TestPrepareRejectsInvalidInputs verifies validation before networking or publication.
func TestPrepareRejectsInvalidInputs(t *testing.T) {
	expected, _ := runtimeFixture(t, nil)
	if err := Prepare(nil, expected, filepath.Join(t.TempDir(), "pack")); err == nil {
		t.Fatal("accepted nil context")
	}
	expected.ID = "../escape"
	if err := Prepare(context.Background(), expected, filepath.Join(t.TempDir(), "pack")); err == nil {
		t.Fatal("accepted unsafe version")
	}
}
