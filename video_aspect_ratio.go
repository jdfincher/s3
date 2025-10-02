package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

type videoAspectRatio struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}

func getVideoAspectRatio(filepath string) (string, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v",
		"error",
		"-print_format",
		"json",
		"-show_streams",
		filepath)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Run()

	vAR := new(videoAspectRatio)
	if err := json.Unmarshal(buf.Bytes(), vAR); err != nil {
		return "", fmt.Errorf("error: unmarshalling video meta data: %w", err)
	}

	denom := gcd(vAR.Streams[0].Width, vAR.Streams[0].Height)
	ratioWidth := vAR.Streams[0].Width / denom
	ratioHeight := vAR.Streams[0].Height / denom

	if ratioWidth > ratioHeight {
		w := ratioWidth / 16
		h := ratioHeight / 9
		if w-denom < 1 && h-denom < 1 {
			return "landscape/", nil
		}
	} else if ratioWidth < ratioHeight {
		w := ratioWidth / 9
		h := ratioHeight / 16
		if w-denom < 1 && h-denom < 1 {
			return "portrait/", nil
		}
	}
	return "other/", nil
}

func gcd(width, height int) int {
	if height == 0 {
		return width
	}
	return gcd(height, width%height)
}
