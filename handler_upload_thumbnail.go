package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

	const maxMemory = 10 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing multipart form request", err)
		return
	}

	file, head, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing file and/or header", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(head.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing media type", err)
		return
	}
	if mediaType != "image/png" && mediaType != "image/jpeg" {
		respondWithError(w, http.StatusBadRequest, "File format not supported, must be .jpg or .png", err)
		return
	}

	fileExtension := mediaType
	fileExtension = strings.Replace(fileExtension, "image/", ".", 1)

	key := make([]byte, 32)
	rand.Read(key)
	fileName := base64.RawURLEncoding.EncodeToString(key)

	localPath := filepath.Join(cfg.assetsRoot, fileName+fileExtension)
	thumbFile, err := os.Create(localPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating file", err)
		return
	}

	if _, err := io.Copy(thumbFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing thumbnail data to file", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s%s", cfg.port, fileName, fileExtension)

	videoMeta, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video id not found in database", err)
		return
	}

	if videoMeta.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unathorized access denied", err)
		return
	}

	videoMeta.ThumbnailURL = &thumbnailURL

	if err = cfg.db.UpdateVideo(videoMeta); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Issue updating video record in database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMeta)
}
