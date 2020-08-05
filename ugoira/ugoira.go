package ugoira

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func unzip(src, dest string) ([]string, error) {
	filenames := make([]string, 0)

	r, err := zip.OpenReader(src)
	if err != nil {
		return filenames, err
	}

	defer r.Close()
	for _, file := range r.File {
		fpath := filepath.Join(dest, file.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return filenames, fmt.Errorf("%s: illegal file path", fpath)
		}

		filenames = append(filenames, fpath)

		if file.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return filenames, err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return filenames, err
		}

		rc, err := file.Open()
		if err != nil {
			return filenames, err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return filenames, err
		}
	}

	return filenames, nil
}

func makeWebm(folder string, u *PixivUgoira) (string, error) {
	var err error
	if u.Duration() < 10.0 {
		err = runCmd("ffmpeg", "-loop", "1", "-framerate", strconv.Itoa(u.FPS()), "-i", folder+"/%06d.jpg", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-t", "10", folder+".mp4")
	} else {
		err = runCmd("ffmpeg", "-framerate", strconv.Itoa(u.FPS()), "-i", folder+"/%06d.jpg", "-c:v", "libx264", "-pix_fmt", "yuv420p", folder+".mp4")
	}

	if err != nil {
		return "", err
	}
	return folder + ".mp4", nil
}

func readAndPrint(r io.Reader) {
	io.Copy(os.Stdout, r)
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	go readAndPrint(stdout)
	go readAndPrint(stderr)

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func UgoiraToGIF(id string) (string, error) {
	u, err := getUgoira(id)
	if err != nil {
		return "", err
	}

	zip, err := downloadZIP(u)
	if err != nil {
		return "", err
	}

	folder := strings.TrimSuffix(zip.Name(), ".zip")
	_, err = unzip(zip.Name(), folder)
	if err != nil {
		return "", err
	}

	webm, err := makeWebm(folder, u)
	if err != nil {
		return "", err
	}
	os.RemoveAll(folder)

	zip.Close()
	os.Remove(zip.Name())
	return webm, nil
}
