package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)
	defer r.Body.Close()

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

	fmt.Println("uploading video file for video", videoID, "by user", userID)

	file, head, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(head.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing media type", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "File Format not supported, must be .mp4", err)
		return
	}

	temp, err := os.CreateTemp("", "tubely_video_upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating file for video upload", err)
		return
	}

	defer os.Remove(temp.Name())
	defer temp.Close()

	_, err = io.Copy(temp, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error copying file contents to memory", err)
		return
	}

	processedPath, err := processVideoForFastStart(temp.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error setting moov atom position", err)
	}

	aspectRatio, err := getVideoAspectRatio(processedPath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to verify aspect ratio", err)
	}

	fStartVideo, err := os.Open(processedPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not find fast start video", err)
	}
	defer os.Remove(fStartVideo.Name())
	defer fStartVideo.Close()

	_, err = fStartVideo.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error setting Seek Start", err)
		return
	}

	fileExtension := ".mp4"
	key := make([]byte, 64)
	rand.Read(key)
	fileName := aspectRatio + base64.RawURLEncoding.EncodeToString(key) + fileExtension

	putObject := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileName,
		Body:        fStartVideo,
		ContentType: &mediaType,
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &putObject)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error saving data to s3 bucket", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video id not found in database", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unathorized access denied", err)
		return
	}

	videoURL := fmt.Sprintf("%s/%s", cfg.s3CfDistribution, fileName)
	video.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video metadata in database", err)
		return
	}
	respondWithJSON(w, http.StatusCreated, video)
}
