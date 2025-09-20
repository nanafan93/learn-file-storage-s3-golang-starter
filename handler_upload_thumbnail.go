package main

import (
	rand2 "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)
	file, header, err := r.FormFile("thumbnail")
	defer file.Close()
	if err != nil {
		respondWithError(w, 400, "too bad", err)
		return
	}
	contentType := header.Header.Get("content-type")
	mimeType, _, err := mime.ParseMediaType(contentType)
	allowedTypes := map[string]bool{
		"image/png":  true,
		"image/jpeg": true,
	}
	if err != nil || !allowedTypes[mimeType] {
		respondWithError(w, http.StatusBadRequest, "bad mime", err)
		return
	}
	extensionForType, err := mime.ExtensionsByType(contentType)
	if extensionForType == nil || err != nil {
		respondWithError(w, http.StatusInternalServerError, "bad mime", err)
		return
	}
	bytes := make([]byte, 32)
	_, randErr := rand2.Read(bytes)
	if randErr != nil {
		respondWithError(w, http.StatusInternalServerError, "too bad", err)
	}
	randomFileName := base64.RawURLEncoding.EncodeToString(bytes)
	filePath := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%s%s", randomFileName, extensionForType[0]))
	newFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "too bad", err)
		return
	}
	_, err = io.Copy(newFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "too bad", err)
		return
	}
	thumbnailUrl := fmt.Sprintf("http://localhost:8091/assets/%s%s", randomFileName, extensionForType[0])
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "too bad", err)
	}
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "cannot find video", err)
		return
	}
	if userID != video.CreateVideoParams.UserID {
		respondWithError(w, http.StatusUnauthorized, "not your video", err)
		return
	}
	video.ThumbnailURL = &thumbnailUrl
	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "sad times", err)
		return
	}
	respondWithJSON(w, http.StatusOK, video)
}
