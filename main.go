package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
)

const MiB = 1 << 20

func main() {
	mux := chi.NewMux()
	
	mux.Handle("/", http.FileServer(http.Dir("./web")))
	mux.Post("/upload-chunk", uploadFileHanlder)

	srv := http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	log.Fatalf("Failed to start server: %s", srv.ListenAndServe())
}

func uploadFileHanlder(w http.ResponseWriter, r *http.Request) {
	filename := r.FormValue("filename")
	totalChunk, _ := strconv.Atoi(r.FormValue("totalChunk"))
	chunkId, _ := strconv.Atoi(r.FormValue("currentChunk"))
	muFile, _, err := r.FormFile("data")

	if err != nil {
		log.Printf("Failed to read form file: %s", err)
		sendResponseJSON(w, http.StatusInternalServerError, nil)
		return
	}
	defer muFile.Close()

	chunkedFile, err := io.ReadAll(muFile)
	if err != nil {
		log.Printf("Failed to read chunk: %s", err)
		sendResponseJSON(w, http.StatusInternalServerError, nil)
		return
	}

	uploadDir := os.Getenv("UPLOAD_DIR")
	tempDir := uploadDir + "/temp"
	err = saveFile(fmt.Sprintf("%s/%s.%d", tempDir, filename, chunkId), chunkedFile)

	if err != nil {
		log.Printf("Failed to save chunked file: %s", err)
		sendResponseJSON(w, http.StatusInternalServerError, nil)
		return
	}

	if chunkId == totalChunk {
		if err := mergeChunkFile(fmt.Sprintf("%s/%s", uploadDir, filename), tempDir, totalChunk, filename); err != nil {
			sendResponseJSON(w, http.StatusInternalServerError, nil)
			return
		}

		if err := cleanupTempFiles(fmt.Sprintf("%s/%s.*", tempDir, filename)); err != nil {
			sendResponseJSON(w, http.StatusInternalServerError, nil)
			return
		}

	}

	sendResponseJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
	})
}

func sendResponseJSON(w http.ResponseWriter, code int, body map[string]interface{}) {
	w.WriteHeader(code)
	err := json.NewEncoder(w).Encode(body)

	if err != nil {
		log.Printf("Failed to encode response body: %s", err)
		return
	}
}

func saveFile(name string, file []byte) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return err
	}

	defer f.Close()
	

	if _, err := f.Write(file); err != nil {
		return err
	}

	return nil
}

func mergeChunkFile(name string, tempDir string, totalChunks int, baseChunkFilename string) error {
	mergedFile, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)

	if err != nil {
		return err
	}
	defer mergedFile.Close()

	for i := 1; i <= totalChunks; i++ {
        tempFile := fmt.Sprintf("%s/%s.%d", tempDir, baseChunkFilename, i)
        chunk, err := os.Open(tempFile)
        if err != nil {
            return err
        }

		defer chunk.Close()

		io.Copy(mergedFile, chunk)
    }

	return nil
}

func cleanupTempFiles(pattern string) error {
	files, _ := filepath.Glob(pattern)
	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			log.Printf("Failed to remove temp file(%s): %s", file, err)
			return err
		}
	}
	return nil
}