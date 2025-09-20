package main

import (
	"context"
	rand2 "crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
	"io"
	"mime"
	"net/http"
	"os"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading videoMetadata", videoID, "by user", userID)
	videoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "cannot find videoMetadata", err)
		return
	}
	if userID != videoMetadata.CreateVideoParams.UserID {
		respondWithError(w, http.StatusUnauthorized, "not your videoMetadata", err)
		return
	}
	maxVideoSize := int64(1 << 30)
	r.Body = http.MaxBytesReader(w, r.Body, maxVideoSize)
	videoFile, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "too bad", err)
		return
	}
	defer videoFile.Close()
	contentType := header.Header.Get("content-type")
	mimeType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mimeType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "bad mime", err)
		return
	}
	tempFile, _ := os.CreateTemp("", "video-upload*")
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	io.Copy(tempFile, videoFile)
	tempFile.Seek(0, io.SeekStart)
	randomFileName, err := getRandomFileName()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "bad", err)
	}
	objectKey := fmt.Sprintf("%s.mp4", randomFileName)
	opts := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &objectKey,
		Body:        tempFile,
		ContentType: &mimeType,
	}
	_, err = cfg.s3Client.PutObject(context.TODO(), &opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "s3 bad", err)
	}
	publicUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, objectKey)
	videoMetadata.VideoURL = &publicUrl
	if err := cfg.db.UpdateVideo(videoMetadata); err != nil {
		respondWithError(w, http.StatusInternalServerError, "sad times", err)
		return
	}
	respondWithJSON(w, http.StatusOK, videoMetadata)
}

func getRandomFileName() (string, error) {
	bytes := make([]byte, 32)
	_, randErr := rand2.Read(bytes)
	if randErr != nil {
		return "", randErr
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
