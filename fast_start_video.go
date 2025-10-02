package main

import (
	"fmt"
	"os/exec"
)

func processVideoForFastStart(filepath string) (string, error) {
	outPath := filepath + ".processing"
	cmd := exec.Command(
		"ffmpeg",
		"-i",
		filepath,
		"-c",
		"copy",
		"-movflags",
		"faststart",
		"-f",
		"mp4",
		outPath)
	if err := cmd.Run(); err != nil {
		return filepath, fmt.Errorf("error: ffmpeg encountered an error: %w", err)
	}
	return outPath, nil
}
