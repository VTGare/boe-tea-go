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
			return filenames, fmt.Errorf("%v: illegal file path", fpath)
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

func makeWebm(folder string, u *Ugoira) (string, error) {
	var err error

	//-vf "pad=ceil(iw/2)*2:ceil(ih/2)*2"
	if u.Duration() < 10.0 {
		err = runCmd("ffmpeg", "-loop", "1", "-framerate", strconv.Itoa(u.FPS()), "-i", folder+"/%06d.jpg", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-vf", `pad=ceil(iw/2)*2:ceil(ih/2)*2`, "-t", "10", folder+".mp4")
	} else {
		err = runCmd("ffmpeg", "-framerate", strconv.Itoa(u.FPS()), "-i", folder+"/%06d.jpg", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-vf", `pad=ceil(iw/2)*2:ceil(ih/2)*2`, folder+".mp4")
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
