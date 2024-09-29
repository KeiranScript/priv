package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	baseDir  = "./files"
	BASE_URL = "https://i.kuuichi.xyz"
)

// RandomString generates a random alphanumeric string of a specified length.
func RandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for i := range bytes {
		bytes[i] = charset[int(bytes[i])%len(charset)]
	}
	return string(bytes), nil
}

// FileUploadHandler handles file uploads.
func FileUploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create the files directory if it doesn't exist
	if err := os.MkdirAll(baseDir, os.ModePerm); err != nil {
		http.Error(w, "Could not create directory", http.StatusInternalServerError)
		return
	}

	// Check the file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !strings.Contains(".jpg.jpeg.png.gif.bmp", ext) {
		http.Error(w, "Invalid file type", http.StatusBadRequest)
		return
	}

	// Generate a random filename
	randomFilename, err := RandomString(6)
	if err != nil {
		http.Error(w, "Error generating random filename", http.StatusInternalServerError)
		return
	}
	newFilename := fmt.Sprintf("%s%s", randomFilename, ext)
	filePath := filepath.Join(baseDir, newFilename)

	// Save the uploaded file
	out, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, "Error copying file", http.StatusInternalServerError)
		return
	}

	// Construct URLs for the response
	imageUrl := fmt.Sprintf("%s/f/%s", BASE_URL, newFilename)   // URL for the served image
	rawUrl := fmt.Sprintf("%s/files/%s", BASE_URL, newFilename) // URL for the raw file

	// Respond with the image URL and raw URL in JSON format
	response := map[string]string{
		"imageUrl": imageUrl,
		"rawUrl":   rawUrl,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func serveImageHTML(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/f/")
	filePath := filepath.Join(baseDir, filename)

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	// Get file info to extract necessary metadata
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Prepare metadata for the HTML response
	fileSizeMB := float64(fileInfo.Size()) / (1024 * 1024) // Convert bytes to megabytes
	totalUploads, err := countFiles(baseDir)               // Get total uploads
	if err != nil {
		totalUploads = 0 // Default to 0 if there's an error
	}
	fileDescription := fmt.Sprintf("Size: %.2f MB - Total uploads: %d", fileSizeMB, totalUploads)

	// Construct the full URL for the image
	imageURL := fmt.Sprintf("%s/files/%s", BASE_URL, filename)
	pageURL := fmt.Sprintf("%s/f/%s", BASE_URL, filename)

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*") // Allow all origins or specify your frontend domain

	// Improved meta tags
	metaTitle := filename
	metaDescription := fileDescription
	metaImage := imageURL
	metaURL := pageURL

	html := fmt.Sprintf(`<html>
		<head>
			<title>%s</title>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<meta property="og:title" content="%s">
			<meta property="og:description" content="%s">
			<meta property="og:image" content="%s">
			<meta property="og:url" content="%s">
			<meta name="twitter:card" content="summary_large_image">
			<meta name="twitter:title" content="%s">
			<meta name="twitter:description" content="%s">
			<meta name="twitter:image" content="%s">
		</head>
		<body>
			<div>
				<img src="%s" alt="%s">
			</div>
		</body>
	</html>`, metaTitle, metaTitle, metaDescription, metaImage, metaURL, metaTitle, metaDescription, metaImage, imageURL, filename)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

func countFiles(dir string) (int, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	return len(files), nil
}

// HomeHandler serves the HTML upload form.
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `<html>
		<body>
			<h1>File Upload</h1>
			<form action="/upload" method="post" enctype="multipart/form-data">
				<input type="file" name="file" required>
				<button type="submit">Upload</button>
			</form>
		</body>
	</html>`)
}

func main() {
	http.HandleFunc("/", HomeHandler)
	http.HandleFunc("/upload", FileUploadHandler)
	http.HandleFunc("/f/", serveImageHTML)

	// Serve files from the ./files directory
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir(baseDir))))

	fmt.Println("Server starting on :5000")
	if err := http.ListenAndServe(":5000", nil); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
	}
}
