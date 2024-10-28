package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"text/template"
	"time"

	"github.com/urfave/cli/v2"
	//"github.com/logrusorgru/aurora"
	"gopkg.in/alessio/shellescape.v1"
)

//go:embed template.rss
var rssTemplate string

var (
	atom                               Atom
	specFile                           string
	awsHandler                         AwsHandler
	askNoQuestions                     bool       = false
	dryRun                             bool       = false
	lameCommandTemplate                string     = defaultLameCommandTemplate
	ffmpegCommandTemplate              string     = defaultFfmpegCommandTemplate
	ffmpegToAudioCommandTemplate       string     = defaultFfmpegToAudioCommandTemplate
	ffmpegPreProcessingCommandTemplate string     = defaultFfmpegPreProcessingCommandTemplate
	templates                          *Templates = &Templates{}
	updateAtom                         bool       = false
	processCounter                     int        = 0
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
	defaultLameCommandTemplate string = `{{ $PRE := "" }}{{ if ne .Atom.LocalStorageDirExpanded "" }}{{ $PRE = print .Atom.LocalStorageDirExpanded "/" }}{{ end }}{{ .Atom.LamepathExpanded }} -b {{ .Atom.Encoding.Bitrate }} --add-id3v2 --tv TLAN={{ escape .Atom.Encoding.Language }} --tt {{ escape .Episode.Title }} --ta {{ escape .Atom.Author }} --tl {{ escape .Atom.Title }} --ty {{ escape (.Episode.PubDate.Format "2006") }} --tc {{ escape .Episode.Subtitle }} --tn {{ .Episode.UID }} --tg {{ escape .Atom.Encoding.Genre }} --ti {{ escape (print $PRE .Atom.Encoding.Coverfront) }} --tv WOAR={{ escape .Atom.Link }} {{ escape (print $PRE .Episode.Input) }} {{ escape (print $PRE .Episode.Output) }}`

	defaultFfmpegCommandTemplate string = `{{ $PRE := ""}}{{ if ne .Atom.LocalStorageDirExpanded ""}}{{ $PRE = print .Atom.LocalStorageDirExpanded "/"}}{{ end }}{{ .Atom.FfmpegpathExpanded }} -y -i {{ escape (print $PRE .Episode.Input) }} -pix_fmt yuv420p -colorspace bt709 -color_trc bt709 -color_primaries bt709 -color_range tv -c:v libx264 -profile:v high -crf {{ .Atom.Encoding.CRF }} -maxrate 1M -bufsize 2M -preset medium -coder 1 -movflags +faststart -x264-params open-gop=0 -c:a libfdk_aac -profile:a aac_low -b:a {{ .Atom.Encoding.ABR }} {{ escape (print $PRE .Episode.Output) }}`

	defaultFfmpegToAudioCommandTemplate string = `{{ $PRE := ""}}{{ if ne .Atom.LocalStorageDirExpanded ""}}{{ $PRE = print .Atom.LocalStorageDirExpanded "/"}}{{ end }}{{ .Atom.FfmpegpathExpanded }} -y -i {{ escape (print $PRE .Episode.Input) }} -vn -f wav -c:a pcm_s16le -ac 2 pipe: | {{ .Atom.LamepathExpanded }} -b {{ .Atom.Encoding.Bitrate }} --add-id3v2 --tv TLAN={{ escape .Atom.Encoding.Language }} --tt {{ escape .Episode.Title }} --ta {{ escape .Atom.Author }} --tl {{ escape .Atom.Title }} --ty {{ escape (.Episode.PubDate.Format "2006") }} --tc {{ escape .Episode.Subtitle }} --tn {{ .Episode.UID }} --tg {{ escape .Atom.Encoding.Genre }} --ti {{ escape (print $PRE .Atom.Encoding.Coverfront) }} --tv WOAR={{ escape .Atom.Link }} - {{ escape (print $PRE .Episode.Output) }}`

	// EQ and compression for the Rode PODMIC
	//
	// The settings should allow you to have a background stereo track
	// (like music) below -10 dB. Minus 10.01 dB in fraction is
	// 0.3158639048423471 or 0.31586 if you can not fit all figures,
	// this should produce a mix without clipping, just make sure you
	// lower the music to this fraction when the vocal track is on.
	defaultFfmpegPreProcessingCommandTemplate string = `ffmpeg -y -i {{ escape .PreProcess.Input }} -vn -ac 2 -filter_complex "` +
		`pan=stereo|c0<.5*c0+.5*c1|c1<.5*c0+.5*c1,` +
		`{{ if .PreProcess.Highpass }}` +
		`highpass=f=90,` +
		`{{ end }}` +
		`{{ if eq .PreProcess.EQ "sm7b" }}` +
		`firequalizer=gain_entry='entry(50,-90); entry(80,-12); entry(125,-2); entry(200, 0)',` +
		`{{ else if eq .PreProcess.EQ "podmic" }}` +
		// `deesser,` +
		`firequalizer=gain_entry='entry(125, +2); entry(250, 0); entry(500, -2); entry(1000, 0); entry(2000, 1); entry(4000, 1); entry(8000, 0); entry(15000, -5)',` +
		`{{ else if eq .PreProcess.EQ "podmic2" }}` +
		// `deesser,` +
		`firequalizer=gain_entry='entry(90,2); entry(538,-3); entry(12000,-2)',` +
		`{{ else if eq .PreProcess.EQ "lowcut" }}` +
		`firequalizer=gain_entry='entry(130,-5); entry(250, 0)',` +
		`{{ end }}` +
		`{{ if eq .PreProcess.Profile "qzj" }}` + `compand=attacks=.001:decays=.5:points=-90/-900|-57/-57|-27/-7|-3/-3|0/-3|20/-3:soft-knee=2,` +
		//`" ` +
		`alimiter=limit=0.7943282347242815:level=disabled" ` +
		`{{ else if eq .PreProcess.Profile "heavy" }}` +
		`compand=attacks=.0001:decays=.5:points=-90/-900|-80/-90|-50/-50|-27/-9|0/-2|20/-2:soft-knee=12,` +
		`alimiter=limit=0.7943282347242815:level=disabled" ` +
		`{{ end }}` +
		`{{ escape (print .PreProcess.Prefix .PreProcess.Input) }}`

	defaultPreProcessingPrefix string = "preprocessed-"
	defaultProfile             string = "qzj"
	defaultEQ                  string = "sm7b"
	defaultHighpass            bool   = false

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
				Name:    "preprocess",
				Aliases: []string{"pre"},
				Usage:   "Run an audiofile (e.g a raw microphone track) through pre-processing",
				Action:  preprocess,
				Flags: []cli.Flag{
					// &cli.StringFlag{
					// 	Name:    "spec",
					// 	Aliases: []string{"s"},
					// 	Value:   defaultSpec,
					// 	Usage:   "Main configuration file for generating the atom RSS",
					// },
					&cli.StringFlag{
						Name:  "prefix",
						Value: defaultPreProcessingPrefix,
						Usage: "Prefix to add to the output file",
					},
					&cli.StringFlag{
						Name:  "profile",
						Value: defaultProfile,
						Usage: "Compression and limiter profile, available: qzj, heavy, none. Limiter settings (except \"none\") will allow you to have background audio/music -10 dB. Minus 10.01 dB in fraction is 0.3158639048423471 or 0.31586 which should produce a mix without clipping.",
					},
					&cli.BoolFlag{
						Name:    "highpass",
						Aliases: []string{"lowcut", "hp"},
						Value:   defaultHighpass,
						Usage:   "Enable highpass/lowcut filter",
					},
					&cli.StringFlag{
						Name:    "equalizer",
						Aliases: []string{"eq"},
						Value:   defaultEQ,
						Usage:   "Pre-configured equalizer settings to apply, available: sm7b, podmic, podmic2, lowcut, none",
					},
				},
			},
			{
				Name:    "parse",
				Aliases: []string{"p"},
				Usage:   "Parse Go template using specification yaml",
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

func preprocess(c *cli.Context) error {
	var err error

	// specFile = c.String("spec")
	// err = loadConfig()
	// if err != nil {
	// 	return err
	// }

	funcMap := template.FuncMap{
		"escape": func(s string) string {
			return shellescape.Quote(s)
		},
	}

	if c.Args().Len() == 0 {
		log.Fatal("You need to specify at least one audiofile as argument(s) to this command")
	}

	// Parse Go template
	templates.FfmpegPreProcessing, err = template.New("ffmpegPreProcessing").Funcs(funcMap).Parse(ffmpegPreProcessingCommandTemplate)
	if err != nil {
		return err
	}

	for _, input := range c.Args().Slice() {
		combined := &Combined{
			// Atom: &atom,
			PreProcess: &PreProcess{
				Input:    input,
				Highpass: c.Bool("highpass"),
				EQ:       c.String("equalizer"),
				Profile:  c.String("profile"),
				Prefix:   c.String("prefix"),
			},
		}
		buf := &bytes.Buffer{}
		err = templates.FfmpegPreProcessing.Execute(buf, combined)
		if err != nil {
			return err
		}
		log.Printf("Executing %s", buf.String())
		cmd := exec.Command(shell, shellCommandOption, buf.String())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("unable to pre-process %s using external tool (ffmpeg): %w", input, err)
		}
	}
	return nil
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

	funcMap := template.FuncMap{
		"escape": func(s string) string {
			return shellescape.Quote(s)
		},
		"timeNow": func() time.Time {
			return time.Now()
		},
		"isAfter": func(t1 time.Time, t2 time.Time) bool {
			if t1.IsZero() || t2.IsZero() {
				return false
			}
			return (t1 == t2 || t1.After(t2))
		},
		"markdown": func(s string) string {
			return MarkdownToHTML(s)
		},
	}

	//t, err := template.ParseFiles(templateFile)
	t, err := template.New("template.rss").Funcs(funcMap).Parse(rssTemplate)
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

	if doAction("Refresh lastBuildDate (will update %s and optionally %s)?", atom.Atom, specFile) {
		atom.LastBuildDate.Time = time.Now().UTC()
		updateAtom = true
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

	// Go template FuncMap, add escape function using
	// gopkg.in/alessio/shellescape.v1.
	funcMap := template.FuncMap{
		"escape": func(s string) string {
			return shellescape.Quote(s)
		},
		"markdown": func(s string) string {
			return MarkdownToHTML(s)
		},
	}

	// Parse Go templates (except the pre-processing command template)
	templates.Lame, err = template.New("lame").Funcs(funcMap).Parse(lameCommandTemplate)
	if err != nil {
		return err
	}
	templates.Ffmpeg, err = template.New("ffmpeg").Funcs(funcMap).Parse(ffmpegCommandTemplate)
	if err != nil {
		return err
	}
	templates.FfmpegToLame, err = template.New("ffmpegToLame").Funcs(funcMap).Parse(ffmpegToAudioCommandTemplate)
	if err != nil {
		return err
	}

	if c.Bool("all") {
		err = processAllEpisodes(templates, askNoQuestions)
		if err != nil {
			return err
		}
	} else {
		err = processEpisodes(templates, c.Args().Slice(), askNoQuestions)
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
