package registry

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/veex-platform/veex-build/pkg/compiler"
	"github.com/veex-platform/veex-build/pkg/vdl"
	"github.com/veex-platform/veex-platform/internal/model"
)

type RegistryHandler struct {
	StorageDir string
	Devices    *DeviceRegistry
}

func (h *RegistryHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Receber o arquivo
	file, header, err := r.FormFile("artifact")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fmt.Printf("📂 Registry: Receiving file %s\n", header.Filename)

	// 2. Metadados
	name := r.FormValue("name")
	version := r.FormValue("version")
	arch := r.FormValue("target_arch")

	artifactID := name + "-" + version
	fileName := artifactID + ".vex"
	filePath := filepath.Join(h.StorageDir, fileName)

	// 3. Salvar no disco
	out, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	size, err := io.Copy(out, file)
	if err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	// 4. Verificar Assinatura (Security Milestone)
	// No MVP, usamos a mesma chave mestre para validação
	if size > 64 {
		content, _ := os.ReadFile(filePath)
		data := content[:len(content)-64]
		sig := content[len(content)-64:]

		seed := make([]byte, 32)
		copy(seed, "veex-master-industrial-key-2026")
		priv := ed25519.NewKeyFromSeed(seed)
		pub := priv.Public().(ed25519.PublicKey)

		if !ed25519.Verify(pub, data, sig) {
			fmt.Println("Registry: REJECTED - Invalid signature!")
			os.Remove(filePath)
			http.Error(w, "Invalid artifact signature", http.StatusUnauthorized)
			return
		}
		fmt.Println("Registry: Artifact signature verified.")
	}

	// 5. Verificar SBOM opcional (Pillar 2: SBOM)
	hasSBOM := false
	sbomFile, _, err := r.FormFile("sbom")
	if err == nil {
		defer sbomFile.Close()
		sbomPath := filepath.Join(h.StorageDir, artifactID+".spdx.json")
		sOut, err := os.Create(sbomPath)
		if err == nil {
			defer sOut.Close()
			io.Copy(sOut, sbomFile)
			hasSBOM = true
			fmt.Printf("Registry: SBOM stored for %s\n", artifactID)
		}
	}

	// 6. Registrar metadados no DB
	baseURL, _ := r.Context().Value("BaseURL").(string)
	artifact := model.Artifact{
		ID:          artifactID,
		Name:        name,
		Version:     version,
		TargetArch:  arch,
		Size:        size,
		CreatedAt:   time.Now(),
		FileName:    fileName,
		DownloadURL: fmt.Sprintf("%s/api/v1/registry/download?id=%s", baseURL, artifactID),
		HasSBOM:     hasSBOM,
	}

	query := `
		INSERT INTO artifacts (id, name, version, target_arch, file_name, download_url)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET 
			version=excluded.version, 
			file_name=excluded.file_name, 
			download_url=excluded.download_url`

	_, err = h.Devices.db.Exec(query, artifact.ID, artifact.Name, artifact.Version, artifact.TargetArch, artifact.FileName, artifact.DownloadURL)
	if err != nil {
		fmt.Printf("❌ Database error during artifact registration: %v\n", err)
	}

	metaPath := filePath + ".json"
	metaFile, _ := os.Create(metaPath)
	json.NewEncoder(metaFile).Encode(artifact)
	metaFile.Close()

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(artifact)
}

func (h *RegistryHandler) Download(w http.ResponseWriter, r *http.Request) {
	artifactID := r.URL.Query().Get("id")
	if artifactID == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(h.StorageDir, artifactID+".vex")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Artifact not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filePath)
}

func (h *RegistryHandler) Build(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		VDL     string `json:"vdl"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 1. Criar VDL temporário
	vdlPath := filepath.Join(os.TempDir(), req.Name+".yaml")
	if err := os.WriteFile(vdlPath, []byte(req.VDL), 0644); err != nil {
		http.Error(w, "Failed to create temp vdl", http.StatusInternalServerError)
		return
	}
	defer os.Remove(vdlPath)

	// 2. Usar a Biblioteca de Build
	def, err := vdl.Load(vdlPath)
	if err != nil {
		http.Error(w, "VDL Load Error: "+err.Error(), http.StatusBadRequest)
		return
	}

	c := compiler.New()
	bin, err := c.Compile(def)
	if err != nil {
		http.Error(w, "Compilation Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Assinar (Opcional, mas recomendado)
	seed := make([]byte, 32)
	copy(seed, "veex-master-industrial-key-2026")
	priv := ed25519.NewKeyFromSeed(seed)
	bin, _ = c.Sign(bin, priv)

	// 4. Salvar .vex
	vexFileName := req.Name + "-" + req.Version + ".vex"
	vexPath := filepath.Join(h.StorageDir, vexFileName)
	if err := os.WriteFile(vexPath, bin, 0644); err != nil {
		http.Error(w, "Failed to save .vex", http.StatusInternalServerError)
		return
	}

	// 5. Salvar metadados no DB
	baseURL, _ := r.Context().Value("BaseURL").(string)
	artifact := model.Artifact{
		ID:          req.Name + "-" + req.Version,
		Name:        req.Name,
		Version:     req.Version,
		TargetArch:  "esp32",
		CreatedAt:   time.Now(),
		FileName:    vexFileName,
		DownloadURL: fmt.Sprintf("%s/api/v1/registry/download?id=%s-%s", baseURL, req.Name, req.Version),
	}

	query := `
		INSERT INTO artifacts (id, name, version, target_arch, file_name, download_url)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET 
			version=excluded.version, 
			file_name=excluded.file_name, 
			download_url=excluded.download_url`

	_, err = h.Devices.db.Exec(query, artifact.ID, artifact.Name, artifact.Version, artifact.TargetArch, artifact.FileName, artifact.DownloadURL)
	if err != nil {
		fmt.Printf("❌ Database error during artifact registration: %v\n", err)
	}

	metaPath := vexPath + ".json"
	metaFile, _ := os.Create(metaPath)
	json.NewEncoder(metaFile).Encode(artifact)
	metaFile.Close()

	fmt.Printf("Cloud Build: Artifact %s created successfully via library.\n", artifact.ID)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(artifact)
}
