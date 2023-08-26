package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path"
	"text/template"
	"time"

	"github.com/urfave/cli/v2"
	//"github.com/logrusorgru/aurora"
)

//go:embed template.rss
var rssTemplate string

var (
	atom Atom
	//origAtom       Rss
	//templateFile          string
	specFile              string
	awsHandler            AwsHandler
	askNoQuestions        bool   = false
	dryRun                bool   = false
	lameCommandTemplate   string = defaultLameCommandTemplate
	ffmpegCommandTemplate string = defaultFfmpegCommandTemplate
	updateAtom            bool   = false
	processCounter        int    = 0
)

const (
	//defaultTemplate   string = "template.rss"
	defaultSpec string = "podspec.yaml"
	//defaultPrivate    string = "private.yaml"
	defaultPodcastRSS string = "podcast.rss"
	// The lame command template is parsed for each episode being
	// encoded where .Atom is the full atom and .Episode is the episode
	// currently being processed (current item in the Episodes struct
	// slice).
	defaultLameCommandTemplate string = `{{ $PRE := "" }}{{ if ne .Atom.LocalStorageDirExpanded "" }}{{ $PRE = print .Atom.LocalStorageDirExpanded "/" }}{{ end }}{{ .Atom.LamepathExpanded }} -b {{ .Atom.Encoding.Bitrate }} --add-id3v2 --tv TLAN="{{ .Atom.Encoding.Language }}" --tt "{{ .Episode.Title }}" --ta "{{ .Atom.Author }}" --tl "{{ .Atom.Title }}" --ty "{{ .Episode.PubDate.Format "2006" }}" --tc "{{ .Episode.Description }}" --tn "{{ .Episode.UID }}" --tg "{{ .Atom.Encoding.Genre }}" --ti "{{ print $PRE .Atom.Encoding.Coverfront }}" --tv WOAR="{{ .Atom.Link }}" "{{ print $PRE .Episode.Input }}" "{{ print $PRE .Episode.Output }}"`

	defaultFfmpegCommandTemplate string = `{{ $PRE := ""}}{{ if ne .Atom.LocalStorageDirExpanded ""}}{{ $PRE = print .Atom.LocalStorageDirExpanded "/"}}{{ end }}{{ .Atom.FfmpegpathExpanded }} -y -i "{{ print $PRE .Episode.Input }}" -pix_fmt yuv420p -colorspace bt709 -color_trc bt709 -color_primaries bt709 -color_range tv -c:v libx264 -profile:v high -crf {{ .Atom.Encoding.CRF }} -maxrate 1M -bufsize 2M -preset medium -coder 1 -movflags +faststart -x264-params open-gop=0 -c:a libfdk_aac -profile:a aac_low -b:a {{ .Atom.Encoding.ABR }} "{{ print $PRE .Episode.Output }}"`

	//		defaultFfmpegCommandTemplate string = `{{ $PRE := ""}}{{ if ne .private.LocalStorageDir ""}}{{ $PRE = print .private.LocalStorageDir "/"}}{{ end }}{{ .atom.Encoding.Ffmpegpath }} -y -i "{{ print $PRE .episode.Input }}" -r 25 -pix_fmt yuv420p -colorspace bt709 -color_trc bt709 -color_primaries bt709 -color_range tv -c:v libx264 -profile:v high -crf {{ .atom.Encoding.CRF }} -g 12 -bf 2 -maxrate 1M -bufsize 2M -preset medium -coder 1 -movflags +faststart -x264-params open-gop=0 -c:a libfdk_aac -profile:a aac_low -b:a {{ .atom.Encoding.ABR }} "{{ print $PRE .episode.Output }}"`

	shell              string = "/bin/sh"
	shellCommandOption string = "-c"
)

func main() {
	app := &cli.App{
		Name:      "mkpod",
		Usage:     "Tool to render a podcast rss feed from spec, automate mp3/mp4 encoding and publish to Amazon S3.",
		Copyright: "Copyright SA6MWA 2022-2023 sa6mwa@gmail.com, https://github.com/sa6mwa/mkpod",
		Commands: []*cli.Command{
			{
				Name:    "parse",
				Aliases: []string{"p"},
				Usage:   "Parse Go template using public and private specification yaml",
				Action:  parser,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "spec",
						Aliases: []string{"s"},
						Value:   defaultSpec,
						Usage:   "Main configuration file for generating the atom RSS",
					},
					// &cli.StringFlag{
					// 	Name:    "private",
					// 	Aliases: []string{"p"},
					// 	Value:   defaultPrivate,
					// 	Usage:   "Secondary configuration file that can be used in the template (usually not publicly checked in)",
					// },
					// &cli.StringFlag{
					// 	Name:    "template",
					// 	Aliases: []string{"t"},
					// 	Value:   defaultTemplate,
					// 	Usage:   "File to use as the Go template to render the atom rss+xml output",
					// },
					&cli.StringFlag{
						Name:    "atom",
						Aliases: []string{"o"},
						Value:   defaultPodcastRSS,
						Usage:   fmt.Sprintf("Atom RSS file to write under the localStorageDir specified in %s", defaultSpec),
					},
					&cli.BoolFlag{
						Name:    "upload",
						Aliases: []string{"u"},
						Value:   false,
						Usage:   fmt.Sprintf("Upload %s to \"output\" Amazon AWS S3 bucket defined in %s", defaultPodcastRSS, defaultSpec),
					},
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Value:   false,
						Usage:   "Force, do not ask if to proceed with an action, just do it",
					},
					&cli.BoolFlag{
						Name:    "dry-run",
						Aliases: []string{"n"},
						Value:   false,
						Usage:   fmt.Sprintf("Behaves like the force option without modifying or producing anything. Will output %s to stdout instead of file", defaultPodcastRSS),
					},
				},
			},
			{
				Name:    "encode",
				Aliases: []string{"e"},
				Usage:   fmt.Sprintf("Encode and upload single or all output files in %s", defaultSpec),
				Action:  encoder,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "spec",
						Aliases: []string{"s"},
						Value:   defaultSpec,
						Usage:   "Main configuration file for generating the atom RSS",
					},
					// &cli.StringFlag{
					// 	Name:    "private",
					// 	Aliases: []string{"p"},
					// 	Value:   defaultPrivate,
					// 	Usage:   "Secondary configuration file that can be used in the template (usually not publicly checked in)",
					// },
					// &cli.StringFlag{
					// 	Name:    "template",
					// 	Aliases: []string{"t"},
					// 	Value:   defaultTemplate,
					// 	Usage:   "File to use as the Go template to render the atom rss+xml output",
					// },
					&cli.BoolFlag{
						Name:    "all",
						Aliases: []string{"a"},
						Value:   false,
						Usage:   "Encode any episode with an empty output filename, missing duration or missing length",
					},
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Value:   false,
						Usage:   fmt.Sprintf("Do not ask whether to re-encode, just do it. Combined with the the \"all\" flag, all episodes in %s will be re-encoded", defaultSpec),
					},
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}

func parser(c *cli.Context) error {
	var err error

	specFile = c.String("spec")
	//privateFile = c.String("private")
	//templateFile = c.String("template")
	askNoQuestions = c.Bool("force")
	dryRun = c.Bool("dry-run")

	err = loadConfig()
	if err != nil {
		return err
	}

	if c.Bool("upload") {
		log.Printf("About to generate %s and upload to S3 bucket %s", atom.Atom, atom.Config.Aws.Buckets.Output)
	} else {
		log.Printf("About to generate %s", atom.Atom)
	}

	//t, err := template.ParseFiles(templateFile)
	t, err := template.New("template.rss").Parse(rssTemplate)
	if err != nil {
		return err
	}

	// We need an AWS session and localStorageDir prior to calling validateAtom().
	awsHandler.NewSession()
	err = createLocalStorageDir()
	if err != nil {
		return err
	}
	err = validateAtom()
	if err != nil {
		return err
	}

	if doAction("Refresh lastBuildDate of atom (%s, will not update %s)?", atom.Atom, specFile) {
		atom.LastBuildDate.Time = time.Now().UTC()
	}

	switch {
	case dryRun && isTerminal() && yes("Write %s to stdout?", atom.Atom):
		fallthrough
	case dryRun && !isTerminal():
		fallthrough
	case !dryRun:
		f := os.Stdout
		if !dryRun {
			f, err = os.Create(atom.Atom)
			if err != nil {
				return err
			}
		}
		//log.Printf("Parsing template %s to %s", templateFile, f.Name())
		log.Printf("Parsing rss template to %s", f.Name())
		err = t.Execute(f, Combined{Atom: &atom})
		if err != nil {
			if !dryRun {
				f.Close()
			}
			return err
		}
		if !dryRun {
			err = f.Close()
			if err != nil {
				return err
			}
		}
		log.Printf("Successfully generated %s", atom.Atom)
	}

	if err := awsHandler.Diff(atom.Config.Aws.Buckets.Output, atom.Atom, atom.Atom); err != nil {
		return err
	}

	// Upload atom file to output S3 bucket.
	if c.Bool("upload") {
		if doAction("Upload new %s?", atom.Atom) {
			if !dryRun {
				err = awsHandler.Upload(atom.Config.Aws.Buckets.Output, atom.Atom, "text/xml", atom.Atom)
				if err != nil {
					return err
				}
			} else {
				log.Printf("Uploading %s to s3://%s", atom.Atom, path.Join(atom.Config.Aws.Buckets.Output, atom.Atom))
			}
		}
	}
	return nil
}

func encoder(c *cli.Context) error {
	var err error

	if c.Args().Len() == 0 && !c.Bool("all") {
		log.Fatal("You need to select one or several episode UIDs to encode as argument(s) to this command or use the all-option -a")
	}

	specFile = c.String("spec")
	//privateFile = c.String("private")
	//templateFile = c.String("template")
	askNoQuestions = c.Bool("force")

	err = loadConfig()
	if err != nil {
		return err
	}

	awsHandler.NewSession()

	err = createLocalStorageDir()
	if err != nil {
		return err
	}
	err = basicAtomValidation()
	if err != nil {
		return err
	}

	lameTemplate, err := template.New("lame").Parse(lameCommandTemplate)
	if err != nil {
		return err
	}

	ffmpegTemplate, err := template.New("ffmpeg").Parse(ffmpegCommandTemplate)
	if err != nil {
		return err
	}

	if c.Bool("all") {
		err = processAllEpisodes(lameTemplate, ffmpegTemplate, askNoQuestions)
		if err != nil {
			return err
		}
	} else {
		err = processEpisodes(lameTemplate, ffmpegTemplate, c.Args().Slice(), askNoQuestions)
		if err != nil {
			return err
		}
	}

	if processCounter == 0 {
		log.Printf("No episode was processed")
	} else {
		plural := ""
		if processCounter > 1 {
			plural = "s"
		}
		log.Printf("Processed %d episode%s", processCounter, plural)
	}

	if updateAtom {
		if doAction("Fields in the atom has changed, re-write %s?", specFile) {
			atom.LastBuildDate.Time = time.Now().UTC()
			f, err := os.Create(specFile)
			if err != nil {
				return fmt.Errorf("unable to re-write %s: %w", specFile, err)
			}
			defer f.Close()
			b, err := atom.Yaml()
			if err != nil {
				return fmt.Errorf("unable to marshall yaml: %w", err)
			}
			_, err = f.Write(b)
			if err != nil {
				return fmt.Errorf("unable to re-write %s: %w", specFile, err)
			}
		}
	}
	return nil
}
