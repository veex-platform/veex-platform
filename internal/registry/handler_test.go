package registry

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/veex-platform/veex-platform/internal/model"
)

func TestUploadArtifact(t *testing.T) {
	// Setup temporary storage
	tmpDir, err := os.MkdirTemp("", "veex-registry-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	handler := &RegistryHandler{
		StorageDir: tmpDir,
	}

	// 1. Prepare dummy artifact data with signature
	data := []byte("vdl-compiled-binary-content")
	seed := make([]byte, 32)
	copy(seed, "veex-master-industrial-key-2026")
	priv := ed25519.NewKeyFromSeed(seed)
	signature := ed25519.Sign(priv, data)
	artifactContent := append(data, signature...)

	// 2. Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, _ := writer.CreateFormFile("artifact", "test.vex")
	part.Write(artifactContent)

	writer.WriteField("name", "test-app")
	writer.WriteField("version", "1.0.0")
	writer.WriteField("target_arch", "esp32")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	// 3. Run handler
	handler.Upload(rr, req)

	// 4. Assertions
	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %v: %s", rr.Code, rr.Body.String())
	}

	var artifact model.Artifact
	json.NewDecoder(rr.Body).Decode(&artifact)

	if artifact.Name != "test-app" {
		t.Errorf("expected name test-app, got %s", artifact.Name)
	}

	// Verify file exists on disk
	expectedPath := filepath.Join(tmpDir, "test-app-1.0.0.vex")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("artifact file was not saved to storage")
	}
}

func TestUploadWithSBOM(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "veex-sbom-test-*")
	defer os.RemoveAll(tmpDir)

	handler := &RegistryHandler{StorageDir: tmpDir}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Artifact
	aPart, _ := writer.CreateFormFile("artifact", "test.vex")
	aPart.Write([]byte("minimal-content-that-fails-sig-but-is-short")) // Short enough to skip sig check in current handler logic (>64 condition)

	// SBOM
	sPart, _ := writer.CreateFormFile("sbom", "test.spdx.json")
	sPart.Write([]byte(`{"spdxVersion": "SPDX-2.3"}`))

	writer.WriteField("name", "sbom-project")
	writer.WriteField("version", "2.1.0")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	handler.Upload(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("unexpected code %v", rr.Code)
	}

	var artifact model.Artifact
	json.NewDecoder(rr.Body).Decode(&artifact)

	if !artifact.HasSBOM {
		t.Error("expected HasSBOM to be true")
	}

	// Verify SBOM file exists
	sbomPath := filepath.Join(tmpDir, "sbom-project-2.1.0.spdx.json")
	if _, err := os.Stat(sbomPath); os.IsNotExist(err) {
		t.Error("SBOM file was not saved")
	}
}

func TestBuildHandler(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "veex-build-test-*")
	defer os.RemoveAll(tmpDir)

	handler := &RegistryHandler{StorageDir: tmpDir}

	reqBody := map[string]string{
		"name":    "cloud-build-test",
		"version": "0.5.0",
		"vdl":     "name: cloud-build-test\nflows:\n  - name: main\n    steps:\n      - name: init\n        capability: platform.core\n        action: wait",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/build", bytes.NewBuffer(jsonBody))
	rr := httptest.NewRecorder()

	handler.Build(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %v: %s", rr.Code, rr.Body.String())
	}

	// Verify artifact has signature (should be > 64 bytes if signed)
	filePath := filepath.Join(tmpDir, "cloud-build-test-0.5.0.vex")
	info, _ := os.Stat(filePath)
	if info.Size() <= 64 {
		t.Errorf("expected signed artifact (>64 bytes), got %d", info.Size())
	}
}
