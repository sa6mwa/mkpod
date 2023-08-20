package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/alfg/mp4"
	"github.com/sa6mwa/mp3duration"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v3"
)

// Generic functions used in more than one cli command of mkpod.go.

func loadConfig() error {
	sf, err := os.Open(specFile)
	if err != nil {
		return err
	}
	atomYaml, err := ioutil.ReadAll(sf)
	if err != nil {
		return err
	}
	sf.Close()
	pcf, err := os.Open(privateFile)
	if err != nil {
		return err
	}
	privateYaml, err := ioutil.ReadAll(pcf)
	if err != nil {
		return err
	}
	pcf.Close()
	err = yaml.Unmarshal(atomYaml, &atom)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(privateYaml, &private)
	if err != nil {
		return err
	}

	// Resolve initial tilde in localStorageDir property.
	if strings.HasPrefix(private.LocalStorageDir, "~/") {
		dirname, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		private.LocalStorageDir = filepath.Join(dirname, private.LocalStorageDir[2:])
	}

	// Resolve tilde in lamepath.
	if strings.HasPrefix(atom.Encoding.Lamepath, "~/") {
		dirname, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		atom.Encoding.Lamepath = filepath.Join(dirname, atom.Encoding.Lamepath[2:])
	}

	// Resolve tilde in ffmpegpath.
	if strings.HasPrefix(atom.Encoding.Ffmpegpath, "~/") {
		dirname, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		atom.Encoding.Ffmpegpath = filepath.Join(dirname, atom.Encoding.Ffmpegpath[2:])
	}

	return nil
}

// Replaces or adds file extension.
func ReplaceExtension(filename string, newExtension string) (newFilename string) {
	ext := filepath.Ext(filename)
	newFilename = filename[0:len(filename)-len(ext)] + newExtension
	return
}

// Returns basename of filename with extension replaced with .mp3
func ExtensionToBaseMp3(filename string) string {
	baseName := path.Base(filename)
	return ReplaceExtension(baseName, ".mp3")
}

func ExtensionToBaseMp4(filename string) string {
	baseName := path.Base(filename)
	return ReplaceExtension(baseName, ".mp4")
}

func doAction(format string, a ...any) bool {
	if dryRun {
		log.Printf("%s No", fmt.Sprintf(format, a...))
		return false
	}
	if askNoQuestions {
		log.Printf("%s Yes", fmt.Sprintf(format, a...))
		return true
	}
	return yes(format, a...)
}

func isTerminal() bool {
	return terminal.IsTerminal(int(os.Stdout.Fd()))
}

func yes(format string, a ...any) bool {
	if !isTerminal() {
		log.Printf("Stdout is not a terminal, will answer no on question: %s", fmt.Sprintf(format, a...))
		return false
	}
	choice := ""
	prompt := &survey.Select{
		Message: fmt.Sprintf(format, a...),
		Options: []string{"No", "Yes", "Exit program"},
		Default: "Yes",
	}
	survey.AskOne(prompt, &choice)
	switch choice {
	case "", "No":
		return false
	case "Yes":
		return true
	case "Exit program":
		log.Println("Exiting")
		os.Exit(0)
	}
	return false
}

func basicAtomValidation() error {
	if len(atom.Atom) < 1 {
		return fmt.Errorf("atom property in %s must not be empty", specFile)
	}
	if atom.TTL < 1 {
		return fmt.Errorf("ttl must not be 0 in %s", specFile)
	}
	if len(atom.Description) < 1 || len(atom.Title) < 1 {
		return fmt.Errorf("title and description must not be empty in %s", specFile)
	}
	fs, err := os.Stat(atom.Encoding.Lamepath)
	if err != nil {
		return fmt.Errorf("unable to stat %s: %v", atom.Encoding.Lamepath, err)
	}
	if fs.IsDir() {
		return fmt.Errorf("%s is a directory, should be path to the lame audio encoder", atom.Encoding.Lamepath)
	}
	if !(fs.Mode()&0111 != 0) { // !IsExecAny
		return fmt.Errorf("%s is not an executable", atom.Encoding.Lamepath)
	}
	return nil
}

func validateAtom() error {
	err := basicAtomValidation()
	if err != nil {
		return err
	}
	for i, e := range atom.Episodes {
		if e.UID < 1 {
			return fmt.Errorf("uid must be above 0 in %s, in episode with output=%s", specFile, e.Output)
		}
		if len(e.Title) < 1 || len(e.Description) < 1 {
			return fmt.Errorf("title and description for episode with uid %d must not be empty in %s", e.UID, specFile)
		}
		if len(e.Author) < 1 {
			return fmt.Errorf("author must not be empty in episode with uid %d (%s)", e.UID, e.Title)
		}
		if len(e.Output) < 3 {
			return fmt.Errorf("episode with uid %d (%s) does not have an output file, maybe you need to encode one?", e.UID, e.Title)
		}
		if e.Length < 1 {
			log.Printf("WARNING: length field (%s size in bytes) of episode with uid %d (%s) is zero.", e.Output, e.UID, e.Title)
			if doAction("Ask AWS for the ContentLength of s3://%s?", path.Join(private.Aws.Buckets.Output, e.Output)) {
				size, err := awsHandler.GetSize(private.Aws.Buckets.Output, e.Output)
				if err != nil {
					return err
				}
				log.Printf("Size of s3://%s is %d (%s will be updated)", path.Join(private.Aws.Buckets.Output, e.Output), size, specFile)
				atom.Episodes[i].Length = size
				updateAtom = true
			}
		}
		if e.Duration.Duration < (time.Duration(1) * time.Second) {
			log.Printf("WARNING: duration is too short for episode with uid %d (%s).", e.UID, e.Title)
			if doAction("Download s3://%s and resolve duration?", path.Join(private.Aws.Buckets.Output, e.Output)) {
				err := awsHandler.Download(private.Aws.Buckets.Output, e.Output)
				if err != nil {
					return err
				}
				di, err := mp3duration.ReadFile(path.Join(private.LocalStorageDir, e.Output))
				if err != nil {
					return err
				}
				log.Printf("%s is %s (HH:MM:SS) long and %d bytes (updating %s).", e.Output, di.Duration, di.Length, specFile)
				atom.Episodes[i].Length = di.Length
				atom.Episodes[i].Duration.Duration = di.TimeDuration
				updateAtom = true
			}
		}
	}
	return nil
}

// Returns a struct combining full atom, private and the episode (for use with
// the lameCommandTemplate or ffmpegCommandTemplate).
func getCombined(episode Episode) map[string]interface{} {
	return map[string]interface{}{
		"atom":    atom,
		"private": private,
		"episode": episode,
	}
}

// This function downloads a single episode's (selected by UID) input file,
// encodes it to mp3, resolves the mp3 files length and duration, and uploads it
// to the output S3 bucket.
func downloadEncodeUpload(lameTemplate *template.Template, ffmpegTemplate *template.Template, uid int64, force bool) error {
	if idx := atom.ContainsEpisode(uid); idx >= 0 {
		if len(atom.Episodes[idx].Output) < 3 || force {
			// Download input file, encode it and upload the output file.
			if doAction("Download s3://%s, encode and upload s3://%s?", path.Join(private.Aws.Buckets.Input, atom.Episodes[idx].Input), path.Join(private.Aws.Buckets.Output, ExtensionToBaseMp3(atom.Episodes[idx].Input))) {
				// Start by downloading the artwork.
				err := awsHandler.Download(private.Aws.Buckets.Input, atom.Episodes[idx].Image)
				if err != nil {
					return err
				}
				err = awsHandler.Download(private.Aws.Buckets.Input, atom.Episodes[idx].Input)
				if err != nil {
					return err
				}

				inputPath := path.Join(private.LocalStorageDir, atom.Episodes[idx].Input)
				inputContentType, err := GetFileContentType(inputPath)
				if err != nil {
					return err
				}

				// if input content type is video/*, we are to encode it using ffmpeg to an mp4.
				if strings.HasPrefix(inputContentType, "video/") {
					atom.Episodes[idx].Output = ExtensionToBaseMp4(atom.Episodes[idx].Input)
					updateAtom = true
					combined := getCombined(atom.Episodes[idx])
					buf := &bytes.Buffer{}
					err = ffmpegTemplate.Execute(buf, combined)
					if err != nil {
						return err
					}
					log.Printf("Executing: %s", buf.String())
					cmd := exec.Command(shell, shellCommandOption, buf.String())
					cmd.Stdin = os.Stdin
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = cmd.Run()
					if err != nil {
						return fmt.Errorf("unable to encode using external encoder (ffmpeg): %w", err)
					}
					// Update atom with the length and duration of the encoded mp4.
					size, duration, err := Mp4Duration(path.Join(private.LocalStorageDir, atom.Episodes[idx].Output))
					if err != nil {
						return err
					}
					log.Printf("%s is %s (HH:MM:SS) long and %d bytes (updating %s)", atom.Episodes[idx].Output, duration, size, specFile)
					atom.Episodes[idx].Length = size
					atom.Episodes[idx].Duration.Duration = duration
					// Upload output mp4 to output S3 bucket.
					contentType, err := GetFileContentType(path.Join(private.LocalStorageDir, atom.Episodes[idx].Output))
					if err != nil {
						return fmt.Errorf("unable to get content-type of file %s: %w", path.Join(private.LocalStorageDir, atom.Episodes[idx].Output), err)
					}
					log.Printf("Content-Type of %s is: %s", atom.Episodes[idx].Output, contentType)
					err = awsHandler.Upload(private.Aws.Buckets.Output, atom.Episodes[idx].Output, contentType, path.Join(private.LocalStorageDir, atom.Episodes[idx].Output))
					if err != nil {
						return err
					}
				} else {
					// ...else, assume it's audio only and encode it to mp3 using lame...

					// lameTemplate uses the output file field in the atom, therefore we
					// need to set it before executing the template.
					atom.Episodes[idx].Output = ExtensionToBaseMp3(atom.Episodes[idx].Input)
					updateAtom = true
					combined := getCombined(atom.Episodes[idx])
					buf := &bytes.Buffer{}
					err = lameTemplate.Execute(buf, combined)
					if err != nil {
						return err
					}
					log.Printf("Executing: %s", buf.String())
					cmd := exec.Command(shell, shellCommandOption, buf.String())
					cmd.Stdin = os.Stdin
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = cmd.Run()
					if err != nil {
						return fmt.Errorf("unable to encode using external encoder (lame): %w", err)
					}
					// Update atom with the length and duration of the encoded mp3.
					di, err := mp3duration.ReadFile(path.Join(private.LocalStorageDir, atom.Episodes[idx].Output))
					if err != nil {
						return err
					}
					log.Printf("%s is %s (HH:MM:SS) long and %d bytes (updating %s)", atom.Episodes[idx].Output, di.Duration, di.Length, specFile)
					atom.Episodes[idx].Length = di.Length
					atom.Episodes[idx].Duration.Duration = di.TimeDuration
					// Upload output mp3 to output S3 bucket.
					contentType, err := GetFileContentType(path.Join(private.LocalStorageDir, atom.Episodes[idx].Output))
					if err != nil {
						return fmt.Errorf("unable to get content-type of file %s: %w", path.Join(private.LocalStorageDir, atom.Episodes[idx].Output), err)
					}
					log.Printf("Content-Type of %s is: %s", atom.Episodes[idx].Output, contentType)
					err = awsHandler.Upload(private.Aws.Buckets.Output, atom.Episodes[idx].Output, contentType, path.Join(private.LocalStorageDir, atom.Episodes[idx].Output))
					if err != nil {
						return err
					}
				}

				// Upload artwork (data-in is free, so I did not bother making a smart upload function)
				contentType, err := GetFileContentType(path.Join(private.LocalStorageDir, atom.Episodes[idx].Image))
				if err != nil {
					return fmt.Errorf("unable to get content-type of file %s: %w", path.Join(private.LocalStorageDir, atom.Episodes[idx].Image), err)
				}
				log.Printf("Content-Type of %s is: %s", atom.Episodes[idx].Image, contentType)
				err = awsHandler.Upload(private.Aws.Buckets.Output, atom.Episodes[idx].Image, contentType, path.Join(private.LocalStorageDir, atom.Episodes[idx].Image))
				if err != nil {
					return err
				}
				processCounter++
			}
		}
	} else {
		log.Printf("WARNING: Episode with uid %d does not exist in %s, skipping", uid, specFile)
	}
	return nil
}

// Function will iterate all episodes and download, encode, upload any episode
// with an empty output filename.
func processAllEpisodes(lameTemplate *template.Template, ffmpegTemplate *template.Template, force bool) error {
	// We need to download the coverfront image in order to encode anything.
	err := awsHandler.Download(private.Aws.Buckets.Input, atom.Encoding.Coverfront)
	if err != nil {
		return err
	}
	for idx, _ := range atom.Episodes {
		err := downloadEncodeUpload(lameTemplate, ffmpegTemplate, atom.Episodes[idx].UID, force)
		if err != nil {
			return err
		}
	}
	return nil
}

func processEpisodes(lameTemplate *template.Template, ffmpegTemplate *template.Template, uidStrings []string, force bool) error {
	// We need to download the coverfront image in order to encode anything.
	err := awsHandler.Download(private.Aws.Buckets.Input, atom.Encoding.Coverfront)
	if err != nil {
		return err
	}
	for _, uidstr := range uidStrings {
		uid, err := strconv.ParseInt(uidstr, 10, 64)
		if err != nil {
			return fmt.Errorf("must specify the UID integer of the episode to process: %w", err)
		}
		err = downloadEncodeUpload(lameTemplate, ffmpegTemplate, uid, force)
		if err != nil {
			return fmt.Errorf("error processing episode with UID %d: %w", uid, err)
		}
	}
	return nil
}

// Returns true if string is in string slice
func strSliceContains(slice []string, str string) bool {
	for idx, _ := range slice {
		if slice[idx] == str {
			return true
		}
	}
	return false
}

func createLocalStorageDir() error {
	dirPaths := []string{path.Dir(path.Join(private.LocalStorageDir, atom.Atom))}
	if len(atom.Encoding.Coverfront) > 0 {
		dirPaths = append(dirPaths, path.Dir(path.Join(private.LocalStorageDir, atom.Encoding.Coverfront)))
	}
	for _, e := range atom.Episodes {
		if len(e.Output) != 0 {
			dirToAdd := path.Dir(path.Join(private.LocalStorageDir, e.Output))
			if !strSliceContains(dirPaths, dirToAdd) {
				dirPaths = append(dirPaths, dirToAdd)
			}
		}
		if len(e.Input) != 0 {
			dirToAdd := path.Dir(path.Join(private.LocalStorageDir, e.Input))
			if !strSliceContains(dirPaths, dirToAdd) {
				dirPaths = append(dirPaths, dirToAdd)
			}
		}
	}
	log.Printf("Creating directories if they do not exist: %s", strings.Join(dirPaths, ", "))
	for _, dir := range dirPaths {
		err := os.MkdirAll(dir, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}

// https://www.tutorialspoint.com/how-to-detect-the-content-type-of-a-file-in-golang
func GetFileContentType(filename string) (contentType string, err error) {
	// to sniff the content type only the first
	// 512 bytes are used.
	var f *os.File
	f, err = os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()
	buf := make([]byte, 512)
	_, err = f.Read(buf)
	if err != nil {
		return
	}
	// the function that actually does the trick
	contentType = http.DetectContentType(buf)
	return
}

// Mp4Duration returns the length in bytes and the duration in
// time.Duration.
func Mp4Duration(filename string) (int64, time.Duration, error) {
	f, err := os.Open(filename)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, 0, err
	}
	mp4, err := mp4.OpenFromReader(f, info.Size())
	if err != nil {
		return 0, 0, err
	}
	return info.Size(), time.Duration(mp4.Moov.Mvhd.Duration), nil
}
