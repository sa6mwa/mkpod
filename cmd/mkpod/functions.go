package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/alfg/mp4"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	mdp "github.com/gomarkdown/markdown/parser"
	"github.com/sa6mwa/id3v24"
	"github.com/sa6mwa/mp3duration"
	"golang.org/x/term"
	"gopkg.in/alessio/shellescape.v1"
	"gopkg.in/yaml.v3"
)

// Generic functions used in more than one cli command of mkpod.go.

// resolvetilde returns path where initial tilde (~) is replaced by
// os.UserHomeDir().
func resolvetilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		dirname, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		return filepath.Join(dirname, path[2:])
	}
	return path
}

func loadConfig() error {
	sf, err := os.Open(specFile)
	if err != nil {
		return err
	}
	atomYaml, err := io.ReadAll(sf)
	sf.Close()
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(atomYaml, &atom)
	if err != nil {
		return err
	}

	// Set defaults

	if atom.Encoding.CRF == 0 {
		atom.Encoding.CRF = 28
	}
	if strings.TrimSpace(atom.Encoding.ABR) == "" {
		atom.Encoding.ABR = "196k"
	}

	return nil
}

// MarkdownToHTML

func MarkdownToHTML(md string) (outputHTML string) {
	// Generate html from all description fields

	p := mdp.NewWithExtensions(mdp.CommonExtensions | mdp.AutoHeadingIDs | mdp.NoEmptyLineBeforeBlock)
	doc := p.Parse([]byte(md))
	renderer := html.NewRenderer(html.RendererOptions{
		Flags: html.CommonFlags | html.HrefTargetBlank,
	})
	outputHTML = string(markdown.Render(doc, renderer))
	return
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

// This can be insecure as format is used unescaped and is only used
// where format is known.
func ExtensionToBaseFormat(filename string, format string) string {
	baseName := path.Base(filename)
	return ReplaceExtension(baseName, "."+strings.TrimSpace(strings.ToLower(format)))
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
	return term.IsTerminal(int(os.Stdout.Fd()))
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
	// Validate executables
	executables := []string{atom.LamepathExpanded(), atom.FFmpegPathExpanded()}
	for _, e := range executables {
		fs, err := os.Stat(e)
		if err != nil {
			return fmt.Errorf("unable to stat %s: %v", e, err)
		}
		if fs.IsDir() {
			return fmt.Errorf("%s is a directory, should be path to executable", e)
		}
		if !(fs.Mode()&0111 != 0) { // !IsExecAny
			return fmt.Errorf("%s is not an executable", e)
		}
	}

	// Ensure all episodes have at least a title, description, and pubDate.
	for i, e := range atom.Episodes {
		if e.UID < 1 {
			return fmt.Errorf("uid must be above 0 in %s, in episode with output=%s", specFile, e.Output)
		}
		if len(e.Title) < 1 || len(e.Description) < 1 {
			return fmt.Errorf("title and description for episode with uid %d must not be empty in %s", e.UID, specFile)
		}
		if len(e.Author) < 1 {
			if len(atom.Author) < 1 {
				return fmt.Errorf("author must not be empty in atom, check %s", specFile)
			}
			atom.Episodes[i].Author = atom.Author
			updateAtom = true
		}
		if e.PubDate.IsZero() {
			log.Printf("UID %d (%s) pubDate is zero, setting to time.Now().UTC()", atom.Episodes[i].UID, atom.Episodes[i].Title)
			atom.Episodes[i].PubDate.Time = time.Now().UTC()
			updateAtom = true
		}
		if len(e.Link) < 1 {
			atom.Episodes[i].Link = atom.Link
			updateAtom = true
		}
	}

	return nil
}

func validateAtom() error {
	err := basicAtomValidation()
	if err != nil {
		return err
	}
	for i, e := range atom.Episodes {
		if len(e.Output) < 3 {
			return fmt.Errorf("episode with uid %d (%s) does not have an output file, maybe you need to encode one?", e.UID, e.Title)
		}
		if e.Length < 1 {
			log.Printf("WARNING: length field (%s size in bytes) of episode with uid %d (%s) is zero.", e.Output, e.UID, e.Title)
			if doAction("Ask AWS for the ContentLength of s3://%s?", path.Join(atom.Config.Aws.Buckets.Output, e.Output)) {
				size, err := awsHandler.GetSize(atom.Config.Aws.Buckets.Output, e.Output)
				if err != nil {
					return err
				}
				log.Printf("Size of s3://%s is %d (%s will be updated)", path.Join(atom.Config.Aws.Buckets.Output, e.Output), size, specFile)
				atom.Episodes[i].Length = size
				updateAtom = true
			}
		}
		if e.Duration.Duration < (time.Duration(1) * time.Second) {
			log.Printf("WARNING: duration is too short for episode with uid %d (%s).", e.UID, e.Title)
			if doAction("Download s3://%s and resolve duration?", path.Join(atom.Config.Aws.Buckets.Output, e.Output)) {
				err := awsHandler.Download(atom.Config.Aws.Buckets.Output, e.Output)
				if err != nil {
					return err
				}

				contentType, err := GetFileContentType(path.Join(atom.LocalStorageDirExpanded(), e.Output))
				if err != nil {
					return err
				}
				if strings.HasPrefix(contentType, "video/") {
					// Assume it's an mp4
					l, d, err := Mp4Duration(path.Join(atom.LocalStorageDirExpanded(), e.Output))
					if err != nil {
						return err
					}
					log.Printf("%s is %s long and %d bytes (updating %s).", e.Output, d, l, specFile)
					atom.Episodes[i].Length = l
					atom.Episodes[i].Duration.Duration = d
					updateAtom = true
				} else {
					// Assume it's an mp3
					di, err := mp3duration.ReadFile(path.Join(atom.LocalStorageDirExpanded(), e.Output))
					if err != nil {
						return err
					}
					log.Printf("%s is %s long and %d bytes (updating %s).", e.Output, di.Duration, di.Length, specFile)
					atom.Episodes[i].Length = di.Length
					atom.Episodes[i].Duration.Duration = di.TimeDuration
					updateAtom = true
				}
			}
		}
	}
	return nil
}

// Returns a struct combining full atom, private and the episode (for use with
// the lameCommandTemplate or ffmpegCommandTemplate).
func getCombined(episode Episode) Combined {
	return Combined{
		Atom:    &atom,
		Episode: &episode,
	}
}

// This function downloads a single episode's (selected by UID) input file,
// encodes it to mp3, resolves the mp3 files length and duration, and uploads it
// to the output S3 bucket.
func downloadEncodeUpload(tmpl *Templates, uid int64, force bool) error {
	if idx := atom.ContainsEpisode(uid); idx >= 0 {
		if strings.TrimSpace(atom.Episodes[idx].Input) == "" {
			return fmt.Errorf("input is missing for UID %d (%s)", atom.Episodes[idx].UID, atom.Episodes[idx].Title)
		}
		if len(atom.Episodes[idx].Output) < 3 || force {
			// If -R option is given and user answers yes or supplied the
			// force option, delete remote master.
			if removeRemoteMasterFile && doAction("Remove s3://%s?", path.Join(atom.Config.Aws.Buckets.Input, atom.Episodes[idx].Input)) {
				if err := awsHandler.Remove(atom.Config.Aws.Buckets.Input, atom.Episodes[idx].Input); err != nil {
					return err
				}
			}
			// Download input file, encode it and upload the output file.
			if doAction("Download s3://%s, encode and upload to s3://%s?", path.Join(atom.Config.Aws.Buckets.Input, atom.Episodes[idx].Input), atom.Config.Aws.Buckets.Output) {
				// Start by downloading the artwork.
				if strings.TrimSpace(atom.Episodes[idx].Image) == "" {
					if strings.TrimSpace(atom.Config.DefaultPodImage) == "" {
						return fmt.Errorf("no image defined for UID %d and defaultPodImage is empty in %s", atom.Episodes[idx].UID, specFile)
					}
					log.Printf("Using %s as default episode image", atom.Config.DefaultPodImage)
					atom.Episodes[idx].Image = atom.Config.DefaultPodImage
					updateAtom = true
				}
				err := awsHandler.Download(atom.Config.Aws.Buckets.Input, atom.Episodes[idx].Image)
				if err != nil {
					return err
				}
				err = awsHandler.Download(atom.Config.Aws.Buckets.Input, atom.Episodes[idx].Input)
				if err != nil {
					return err
				}

				inputPath := path.Join(atom.LocalStorageDirExpanded(), atom.Episodes[idx].Input)
				inputContentType, err := GetFileContentType(inputPath)
				if err != nil {
					return err
				}

				format := strings.TrimSpace(strings.ToLower(atom.Episodes[idx].Format))

				// TODO: This nested if statement needs to serious refactoring.
				// Perhaps
				//
				// if input content type is video/* and format is not "audio",
				// we are to encode it using ffmpeg to an mp4. If format is
				// "audio", drop the video stream and encode an mp3 (audio
				// only).
				if strings.HasPrefix(inputContentType, "video/") {
					// If episode.format is video or mp4, it's a video episode.
					switch format {
					case "", "video", "mp4":
						if err := EncodeMP4(tmpl, &atom.Episodes[idx]); err != nil {
							return err
						}
					case "audio":
						if strings.EqualFold(atom.Encoding.PreferredFormat, "m4a") || strings.EqualFold(atom.Encoding.PreferredFormat, "m4b") {
							// Encode into m4a or m4b
							if err := EncodeFFmpegAudio(tmpl, &atom.Episodes[idx], atom.Encoding.PreferredFormat); err != nil {
								return err
							}
						} else {
							// Encode mp3 via ffmpeg (piped into lame)
							if err := EncodeMP3ViaFFmpeg(tmpl, &atom.Episodes[idx]); err != nil {
								return err
							}
						}
					case "mp3":
						// Encode mp3 via ffmpeg
						if err := EncodeMP3ViaFFmpeg(tmpl, &atom.Episodes[idx]); err != nil {
							return err
						}
					case "m4a", "m4b":
						// Encode m4a or m4b
						if err := EncodeFFmpegAudio(tmpl, &atom.Episodes[idx], format); err != nil {
							return err
						}
					default:
						return fmt.Errorf("invalid or unsupported format %q", format)
					}
				} else {
					// ...else, assume it's audio only and encode it to either
					// mp3 using lame or m4a/m4b using ffmpeg
					switch format {
					case "", "audio":
						if strings.EqualFold(atom.Encoding.PreferredFormat, "m4a") || strings.EqualFold(atom.Encoding.PreferredFormat, "m4b") {
							// Encode into m4a or m4b
							if err := EncodeFFmpegAudio(tmpl, &atom.Episodes[idx], atom.Encoding.PreferredFormat); err != nil {
								return err
							}
						} else {
							// Encode mp3 using Lame
							if err := EncodeMP3(tmpl, &atom.Episodes[idx]); err != nil {
								return err
							}
						}
					case "mp3":
						// Encode mp3 using Lame
						if err := EncodeMP3(tmpl, &atom.Episodes[idx]); err != nil {
							return err
						}
					case "m4a", "m4b":
						// Encode m4a or m4b
						if err := EncodeFFmpegAudio(tmpl, &atom.Episodes[idx], format); err != nil {
							return err
						}
					default:
						return fmt.Errorf("invalid or unsupported format %q", format)
					}
				}

				// The Encode functions above all change fields in the atom.
				updateAtom = true

				// Upload output mp4/mp3/m4a/m4b to output S3 bucket.
				contentType, err := GetFileContentType(path.Join(atom.LocalStorageDirExpanded(), atom.Episodes[idx].Output))
				if err != nil {
					return fmt.Errorf("unable to get content-type of file %s: %w", path.Join(atom.LocalStorageDirExpanded(), atom.Episodes[idx].Output), err)
				}
				log.Printf("Content-Type of %s is: %s", atom.Episodes[idx].Output, contentType)
				atom.Episodes[idx].Type = contentType
				err = awsHandler.Upload(atom.Config.Aws.Buckets.Output, atom.Episodes[idx].Output, contentType, path.Join(atom.LocalStorageDirExpanded(), atom.Episodes[idx].Output))
				if err != nil {
					return err
				}

				// Ensure there is a pubDate set
				if atom.Episodes[idx].PubDate.IsZero() {
					log.Printf("UID %d (%s) pubDate is zero, setting to time.Now().UTC()", atom.Episodes[idx].UID, atom.Episodes[idx].Title)
					atom.Episodes[idx].PubDate.Time = time.Now().UTC()
					updateAtom = true
				}

				// Upload artwork (data-in is free, so I did not bother making a smart upload function)
				contentType, err = GetFileContentType(path.Join(atom.LocalStorageDirExpanded(), atom.Episodes[idx].Image))
				if err != nil {
					return fmt.Errorf("unable to get content-type of file %s: %w", path.Join(atom.LocalStorageDirExpanded(), atom.Episodes[idx].Image), err)
				}
				log.Printf("Content-Type of %s is: %s", atom.Episodes[idx].Image, contentType)
				err = awsHandler.Upload(atom.Config.Aws.Buckets.Output, atom.Episodes[idx].Image, contentType, path.Join(atom.LocalStorageDirExpanded(), atom.Episodes[idx].Image))
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
func processAllEpisodes(tmpl *Templates, force bool) error {
	// We need to download the coverfront image in order to encode anything.
	err := awsHandler.Download(atom.Config.Aws.Buckets.Input, atom.Encoding.Coverfront)
	if err != nil {
		return err
	}
	for idx := range atom.Episodes {
		err := downloadEncodeUpload(tmpl, atom.Episodes[idx].UID, force)
		if err != nil {
			return err
		}
	}
	return nil
}

func processEpisodes(tmpl *Templates, uidStrings []string, force bool) error {
	// We need to download the coverfront image in order to encode anything.
	err := awsHandler.Download(atom.Config.Aws.Buckets.Input, atom.Encoding.Coverfront)
	if err != nil {
		return err
	}
	for _, uidstr := range uidStrings {
		uid, err := strconv.ParseInt(uidstr, 10, 64)
		if err != nil {
			return fmt.Errorf("must specify the UID integer of the episode to process: %w", err)
		}
		err = downloadEncodeUpload(tmpl, uid, force)
		if err != nil {
			return fmt.Errorf("error processing episode with UID %d: %w", uid, err)
		}
	}
	return nil
}

// Returns true if string is in string slice
func strSliceContains(slice []string, str string) bool {
	for idx := range slice {
		if slice[idx] == str {
			return true
		}
	}
	return false
}

func createLocalStorageDir() error {
	dirPaths := []string{atom.LocalStorageDirExpanded()}
	if len(atom.Encoding.Coverfront) > 0 {
		dirPaths = append(dirPaths, path.Dir(path.Join(atom.LocalStorageDirExpanded(), atom.Encoding.Coverfront)))
	}
	for _, e := range atom.Episodes {
		if len(e.Output) != 0 {
			dirToAdd := path.Dir(path.Join(atom.LocalStorageDirExpanded(), e.Output))
			if !strSliceContains(dirPaths, dirToAdd) {
				dirPaths = append(dirPaths, dirToAdd)
			}
		}
		if len(e.Input) != 0 {
			dirToAdd := path.Dir(path.Join(atom.LocalStorageDirExpanded(), e.Input))
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
	if mp4 != nil && mp4.Moov != nil && mp4.Moov.Mvhd != nil {
		return info.Size(), time.Duration(mp4.Moov.Mvhd.Duration) * time.Millisecond, nil
	} else {
		return 0, 0, fmt.Errorf("%s does not contain a Moov Mvhd box (maybe not an mp4?)", filename)
	}
}

// FFprobe runs ffprobe on filename and returns an FFprobeJSON with
// format filled in or returns error if something failed. Full command
// executed via shell (probably /bin/sh) and shellCommandOption (-c):
//
//	ffprobe -v error -show_format -print_format json filename
func FFprobe(filename string) (*FFprobeJSON, error) {
	ffprobeCmd := fmt.Sprintf("ffprobe -v error -show_format -print_format json %s", shellescape.Quote(filename))
	cmd := exec.Command(shell, shellCommandOption, ffprobeCmd)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var result FFprobeJSON
	if err := json.NewDecoder(&out).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSizeAndDurationViaFFprobe returns duration, size or error if
// something failed.
func GetSizeAndDurationViaFFprobe(filename string) (time.Duration, int64, error) {
	fi, err := os.Stat(filename)
	if err != nil {
		return 0, 0, err
	}
	ffprobejson, err := FFprobe(filename)
	if err != nil {
		return 0, fi.Size(), err
	}
	return ffprobejson.Format.Duration.Duration, fi.Size(), nil
}

func ApplyMetadataToM4A(metadatafile, inputM4file, outputM4file string) error {
	ffmpegCmd := fmt.Sprintf("ffmpeg -i %s -i %s -map 0 -map_metadata 1 -c copy %s", shellescape.Quote(inputM4file), shellescape.Quote(metadatafile), shellescape.Quote(outputM4file))
	return Run(ffmpegCmd)
}

// Run runs commandString via shell (probably /bin/sh) and
// shellCommandOption (-c). Output is os.Stdout, os.Stderr,
// etc. Returns error if cmd.Run() fails.
func Run(commandString string) error {
	cmd := exec.Command(shell, shellCommandOption, commandString)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// EncodeFFmpegAudio encodes input into an m4a or m4b file depending
// on the value of format.
func EncodeFFmpegAudio(tmpl *Templates, episode *Episode, format string) error {
	if episode == nil {
		return errors.New("received nil pointer episode")
	}
	format = strings.TrimSpace(strings.ToLower(format))
	episode.Output = ExtensionToBaseFormat(episode.Input, format)
	combined := getCombined(*episode)

	rplcr := strings.NewReplacer("\n", " ", "\r", "")

	lang := atom.Encoding.Language
	if episode.EncodingLanguage != "" {
		lang = episode.EncodingLanguage
	}

	trackInfo := id3v24.TrackInfo{
		Title:       episode.Title,
		Album:       atom.Title,
		Artist:      episode.Author,
		Genre:       atom.Encoding.Genre,
		Year:        episode.PubDate.Format("2006"),
		Date:        episode.PubDate.Time,
		Track:       fmt.Sprintf("%d", episode.UID),
		Comment:     episode.Link,
		Description: rplcr.Replace(episode.Subtitle),
		Language:    lang,
		CoverJPEG:   path.Join(atom.LocalStorageDirExpanded(), atom.Encoding.Coverfront),
		Chapters:    episode.Chapters,
	}

	buf := &bytes.Buffer{}
	if err := tmpl.FFmpegM4A.Execute(buf, combined); err != nil {
		return err
	}

	// First, encode to preferred output format (m4a or m4b).
	log.Printf("Executing: %s", buf.String())
	if err := Run(buf.String()); err != nil {
		return fmt.Errorf("unable to encode to %s using ffmpeg: %w", atom.Encoding.PreferredFormat, err)
	}
	temporaryOutputPath := path.Join(atom.LocalStorageDirExpanded(), "without-metadata-"+episode.Output)
	outputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)

	// // Run ffprobe to get duration and size (size is from os.Stat)
	// log.Printf("Retrieving size and duration of %q", temporaryOutputPath)
	// duration, size, err := GetSizeAndDurationViaFFprobe(temporaryOutputPath)
	// if err != nil {
	// 	return fmt.Errorf("unable to get size and duration via ffprobe: %w", err)
	// }

	// Use github.com/alfg/mp4 to get size and duration of temporary m4a
	// or m4b.
	size, duration, err := Mp4Duration(temporaryOutputPath)
	if err != nil {
		return fmt.Errorf("unable to get size and duration: %w", err)
	}
	// Generate metadata /w chapters (if any)
	metadataFile, err := id3v24.WriteFFmpegMetadataFile(duration, trackInfo)
	if err != nil {
		return fmt.Errorf("unable to generate ffmetadata file: %w", err)
	}
	// Apply metadata to output file
	if err := ApplyMetadataToM4A(metadataFile, temporaryOutputPath, outputPath); err != nil {
		return fmt.Errorf("unable to apply metadata %q to %q and create %q: %w", metadataFile, temporaryOutputPath, outputPath, err)
	}
	log.Printf("Removing %q", temporaryOutputPath)
	if err := os.Remove(temporaryOutputPath); err != nil {
		return fmt.Errorf("unable to remove %q: %w", temporaryOutputPath, err)
	}
	// Update episode length and duration
	log.Printf("%s is %s long and %d bytes (updating %s)", episode.Output, duration, size, specFile)
	episode.Length = size
	episode.Duration.Duration = duration
	return nil
}

// EncodeMP3ViaFFmpeg encodes input through ffmpeg piped into lame as
// an mp3.
func EncodeMP3ViaFFmpeg(tmpl *Templates, episode *Episode) error {
	if episode == nil {
		return errors.New("received nil pointer episode")
	}
	episode.Output = ExtensionToBaseMp3(episode.Input)
	combined := getCombined(*episode)

	buf := &bytes.Buffer{}
	if err := tmpl.FFmpegToLame.Execute(buf, combined); err != nil {
		return err
	}
	log.Printf("Executing: %s", buf.String())
	if err := Run(buf.String()); err != nil {
		return fmt.Errorf("unable to encode to audio using external encoders (ffmpeg and lame): %w", err)
	}

	// Add ID3v2.4 tag (artist, album, title, chapters, etc.).
	outputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)
	log.Printf("Adding ID3v2.4 tag to %s", outputPath)
	if err := id3v24.WriteID3v2Tag(outputPath, id3v24.TrackInfo{
		Title:     episode.Title,
		Album:     atom.Title,
		Artist:    episode.Author,
		Genre:     atom.Encoding.Genre,
		Year:      episode.PubDate.Format("2006"),
		CoverJPEG: path.Join(atom.LocalStorageDirExpanded(), atom.Encoding.Coverfront),
		Chapters:  episode.Chapters,
	}); err != nil {
		return err
	}
	// Get duration and length.
	di, err := mp3duration.ReadFile(outputPath)
	if err != nil {
		return err
	}
	// Update atom with the length and duration of the encoded mp3.
	log.Printf("%s is %s long and %d bytes (updating %s)", episode.Output, di.Duration, di.Length, specFile)
	episode.Length = di.Length
	episode.Duration.Duration = di.TimeDuration
	return nil
}

// EncodeMP3 encodes an mp3 using lame.
func EncodeMP3(tmpl *Templates, episode *Episode) error {
	if episode == nil {
		return errors.New("received nil pointer episode")
	}
	episode.Output = ExtensionToBaseMp3(episode.Input)
	combined := getCombined(*episode)
	buf := &bytes.Buffer{}
	if err := tmpl.Lame.Execute(buf, combined); err != nil {
		return err
	}
	log.Printf("Executing: %s", buf.String())
	if err := Run(buf.String()); err != nil {
		return fmt.Errorf("unable to encode to audio using external encoders (ffmpeg and lame): %w", err)
	}
	// Add ID3v2.4 tag (artist, album, title, chapters, etc.).
	outputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)
	log.Printf("Adding ID3v2.4 tag to %s", outputPath)
	if err := id3v24.WriteID3v2Tag(outputPath, id3v24.TrackInfo{
		Title:     episode.Title,
		Album:     atom.Title,
		Artist:    episode.Author,
		Genre:     atom.Encoding.Genre,
		Year:      episode.PubDate.Format("2006"),
		CoverJPEG: path.Join(atom.LocalStorageDirExpanded(), atom.Encoding.Coverfront),
		Chapters:  episode.Chapters,
	}); err != nil {
		return err
	}
	// Update atom with the length and duration of the encoded mp3.
	di, err := mp3duration.ReadFile(outputPath)
	if err != nil {
		return err
	}
	log.Printf("%s is %s long and %d bytes (updating %s)", episode.Output, di.Duration, di.Length, specFile)
	episode.Length = di.Length
	episode.Duration.Duration = di.TimeDuration
	return nil
}

// EncodeMP4 encodes input to an mp4 video.
func EncodeMP4(tmpl *Templates, episode *Episode) error {
	if episode == nil {
		return errors.New("received nil pointer episode")
	}
	episode.Output = ExtensionToBaseMp4(episode.Input)
	combined := getCombined(*episode)
	buf := &bytes.Buffer{}
	if err := tmpl.FFmpeg.Execute(buf, combined); err != nil {
		return err
	}
	log.Printf("Executing: %s", buf.String())
	if err := Run(buf.String()); err != nil {
		return fmt.Errorf("unable to encode to audio using external encoders (ffmpeg and lame): %w", err)
	}
	// Update atom with the length and duration of the encoded mp4
	size, duration, err := Mp4Duration(path.Join(atom.LocalStorageDirExpanded(), episode.Output))
	if err != nil {
		return err
	}
	log.Printf("%s is %s long and %d bytes (updating %s)", episode.Output, duration, size, specFile)
	episode.Length = size
	episode.Duration.Duration = duration
	return nil
}
