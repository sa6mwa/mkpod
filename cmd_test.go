package main

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestCLICommands tests that the main commands are available and respond correctly
func TestCLICommands(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "help shows available commands",
			args:     []string{"--help"},
			expected: "Available Commands:",
		},
		{
			name:     "preprocess help",
			args:     []string{"preprocess", "--help"},
			expected: "mkpod preprocess is a thin utility wrapper",
		},
		{
			name:     "preprocess help shows supported presets",
			args:     []string{"preprocess", "--help"},
			expected: "lowcut",
		},
		{
			name:     "parse help",
			args:     []string{"parse", "--help"},
			expected: "Parse the podcast specification file",
		},
		{
			name:     "encode help",
			args:     []string{"encode", "--help"},
			expected: "Encode and upload episode media defined in the podcast specification",
		},
		{
			name:     "encode help shows remove-remote-master flag",
			args:     []string{"encode", "--help"},
			expected: "--remove-remote-master",
		},
		{
			name:     "init help",
			args:     []string{"init", "--help"},
			expected: "Initialize a new mkpod workspace",
		},
		{
			name:     "root help includes examples",
			args:     []string{"--help"},
			expected: "Examples:",
		},
		{
			name:     "parse help includes upload example",
			args:     []string{"parse", "--help"},
			expected: "mkpod parse --spec ./podcast/podspec.yaml --upload",
		},
		{
			name:     "encode help includes remove-remote-master example",
			args:     []string{"encode", "--help"},
			expected: "mkpod encode --spec ./podcast/podspec.yaml --all --remove-remote-master",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("go", append([]string{"run", "."}, tt.args...)...)
			output, err := cmd.CombinedOutput()
			if err != nil && !strings.Contains(string(output), tt.expected) {
				t.Fatalf("Command failed: %v\nOutput: %s", err, output)
			}

			if !strings.Contains(string(output), tt.expected) {
				t.Errorf("Expected output to contain %q, got: %s", tt.expected, output)
			}
		})
	}
}

// TestCommandAliases tests that command aliases work correctly
func TestCommandAliases(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "preprocess alias pre",
			args:     []string{"pre", "--help"},
			expected: "mkpod preprocess is a thin utility wrapper",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("go", append([]string{"run", "."}, tt.args...)...)
			output, err := cmd.CombinedOutput()
			if err != nil && !strings.Contains(string(output), tt.expected) {
				t.Fatalf("Command failed: %v\nOutput: %s", err, output)
			}

			if !strings.Contains(string(output), tt.expected) {
				t.Errorf("Expected output to contain %q, got: %s", tt.expected, output)
			}
		})
	}
}

func TestInitThenParseDryRun(t *testing.T) {
	target := filepath.Join(t.TempDir(), "podcast")

	if _, err := cmdTest(target, "init", target); err != nil {
		t.Fatalf("mkpod init failed: %v", err)
	}

	output, err := cmdTest(target, "parse", "--spec", filepath.Join(target, "podspec.yaml"), "--dry-run")
	if err != nil {
		t.Fatalf("mkpod parse --dry-run failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "<rss") {
		t.Fatalf("expected RSS output, got: %s", output)
	}
	if !strings.Contains(output, "<title>"+filepath.Base(target)+"</title>") {
		t.Fatalf("expected podcast title in RSS output, got: %s", output)
	}

	specPath := filepath.Join(target, "podspec.yaml")
	before, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec before second dry-run: %v", err)
	}

	if _, err := cmdTest("", "parse", "--spec", specPath, "--dry-run"); err != nil {
		t.Fatalf("second mkpod parse --dry-run failed: %v", err)
	}

	after, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec after second dry-run: %v", err)
	}

	if string(before) != string(after) {
		t.Fatalf("parse --dry-run modified podspec.yaml")
	}
}

func TestEncodeFailsForEpisodeMissingTitle(t *testing.T) {
	target := filepath.Join(t.TempDir(), "podcast")

	if _, err := cmdTest("", "init", target); err != nil {
		t.Fatalf("mkpod init failed: %v", err)
	}

	specPath := filepath.Join(target, "podspec.yaml")
	specContent, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}

	updated := strings.Replace(string(specContent), "episodes: []", `episodes:
    - uid: 1
      pubDate: "Fri, 25 Mar 2022 16:00:13 +0000"
      link: "https://example.com/my-podcast/episodes/1"
      subtitle: "Episode without title"
      description: "This episode is intentionally invalid for testing"
      image: "artwork/podcast-cover.jpg"
      input: "masters/episode.wav"
`, 1)

	if err := os.WriteFile(specPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write updated spec: %v", err)
	}

	output, err := cmdTest("", "encode", "--spec", specPath, "1")
	if err == nil {
		t.Fatalf("mkpod encode unexpectedly succeeded\nOutput: %s", output)
	}
	if !strings.Contains(output, "episode title is required for encoding") {
		t.Fatalf("expected missing title error, got: %s", output)
	}
}

func TestEncodeRequiresUIDOrAll(t *testing.T) {
	output, err := cmdTest("", "encode")
	if err == nil {
		t.Fatalf("mkpod encode unexpectedly succeeded\nOutput: %s", output)
	}
	if !strings.Contains(output, "select one or more episode UIDs or use --all") {
		t.Fatalf("expected encode usage error, got: %s", output)
	}
}

func TestParseRejectsPositionalArguments(t *testing.T) {
	output, err := cmdTest("", "parse", "extra")
	if err == nil {
		t.Fatalf("mkpod parse unexpectedly succeeded\nOutput: %s", output)
	}
	if !strings.Contains(output, "parse does not take positional arguments") {
		t.Fatalf("expected parse argument error, got: %s", output)
	}
}
func TestPreprocessFailsWithoutInputFiles(t *testing.T) {
	output, err := cmdTest("", "preprocess")
	if err == nil {
		t.Fatalf("mkpod preprocess unexpectedly succeeded\nOutput: %s", output)
	}
	if !strings.Contains(output, "provide one or more audio files to preprocess") {
		t.Fatalf("expected preprocess argument error, got: %s", output)
	}
}

func TestParseSkipsInvalidEpisodesButSucceeds(t *testing.T) {
	target := filepath.Join(t.TempDir(), "podcast")

	if _, err := cmdTest("", "init", target); err != nil {
		t.Fatalf("mkpod init failed: %v", err)
	}

	specPath := filepath.Join(target, "podspec.yaml")
	specContent, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}

	updated := strings.Replace(string(specContent), "episodes: []", `episodes:
    - uid: 1
      title: "Good Episode"
      pubDate: "Fri, 25 Mar 2022 16:00:13 +0000"
      link: "https://example.com/my-podcast/episodes/1"
      duration: "00:05:00"
      subtitle: "Valid episode"
      description: "This one should render"
      type: "audio/mpeg"
      length: 12345
      image: "artwork/podcast-cover.jpg"
      output: "episode1.mp3"
    - uid: 2
      title: "Bad Episode"
      pubDate: "Fri, 25 Mar 2022 16:00:13 +0000"
      link: "https://example.com/my-podcast/episodes/2"
      subtitle: "Invalid episode"
      description: "Missing output, duration, type, length, image"
`, 1)

	if err := os.WriteFile(specPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write updated spec: %v", err)
	}

	output, err := cmdTest("", "parse", "--spec", specPath, "--dry-run")
	if err != nil {
		t.Fatalf("mkpod parse unexpectedly failed\nOutput: %s", output)
	}
	if !strings.Contains(output, "Excluding episode from RSS due to missing required fields") {
		t.Fatalf("expected warning about skipped invalid episode, got: %s", output)
	}
	if !strings.Contains(output, "<title>Good Episode</title>") {
		t.Fatalf("expected valid episode in RSS output, got: %s", output)
	}
	if strings.Contains(output, "<title>Bad Episode</title>") {
		t.Fatalf("invalid episode should not be rendered in RSS output: %s", output)
	}
}

func TestParseWithoutForceLeavesSpecUnchangedInNonInteractiveRun(t *testing.T) {
	target := filepath.Join(t.TempDir(), "podcast")

	if _, err := cmdTest("", "init", target); err != nil {
		t.Fatalf("mkpod init failed: %v", err)
	}

	specPath := filepath.Join(target, "podspec.yaml")
	before, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec before parse: %v", err)
	}

	if output, err := cmdTest("", "parse", "--spec", specPath); err != nil {
		t.Fatalf("mkpod parse failed: %v\nOutput: %s", err, output)
	}

	after, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec after parse: %v", err)
	}

	if string(before) != string(after) {
		t.Fatalf("parse without --force rewrote podspec.yaml")
	}
	if _, err := os.Stat(filepath.Join(target, "podcast.rss")); err != nil {
		t.Fatalf("expected podcast.rss to be written: %v", err)
	}
}

func TestParseForceRewritesSpecInNonInteractiveRun(t *testing.T) {
	target := filepath.Join(t.TempDir(), "podcast")

	if _, err := cmdTest("", "init", target); err != nil {
		t.Fatalf("mkpod init failed: %v", err)
	}

	specPath := filepath.Join(target, "podspec.yaml")
	before, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec before force parse: %v", err)
	}

	stale := strings.Replace(string(before), "lastBuildDate:", `lastBuildDate: "Tue, 01 Jan 2002 00:00:00 +0000" # `, 1)
	if stale == string(before) {
		t.Fatal("expected init spec to contain lastBuildDate")
	}
	if err := os.WriteFile(specPath, []byte(stale), 0o644); err != nil {
		t.Fatalf("write stale spec: %v", err)
	}

	if output, err := cmdTest("", "parse", "--spec", specPath, "--force"); err != nil {
		t.Fatalf("mkpod parse --force failed: %v\nOutput: %s", err, output)
	}

	after, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec after force parse: %v", err)
	}

	if string(after) == stale {
		t.Fatalf("parse --force did not rewrite podspec.yaml")
	}
	if !strings.Contains(string(after), "lastBuildDate:") {
		t.Fatalf("rewritten spec missing lastBuildDate: %s", after)
	}
}

func cmdTest(_ string, args ...string) (string, error) {
	command := exec.Command("go", append([]string{"run", "."}, args...)...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func TestParseShowsInvalidYAMLError(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "podspec.yaml")
	if err := os.WriteFile(specPath, []byte("config: [\n"), 0o644); err != nil {
		t.Fatalf("write invalid spec: %v", err)
	}

	output, err := cmdTest("", "parse", "--spec", specPath)
	if err == nil {
		t.Fatalf("mkpod parse unexpectedly succeeded\nOutput: %s", output)
	}
	if !strings.Contains(output, "invalid YAML") {
		t.Fatalf("expected invalid YAML error, got: %s", output)
	}
}

var (
	compiledBinaryOnce sync.Once
	compiledBinaryPath string
	compiledBinaryErr  error
)

func compiledBinary(t *testing.T) string {
	t.Helper()
	compiledBinaryOnce.Do(func() {
		binDir, err := os.MkdirTemp("", "mkpod-binary-*")
		if err != nil {
			compiledBinaryErr = err
			return
		}
		compiledBinaryPath = filepath.Join(binDir, "mkpod-test")
		cmd := exec.Command("go", "build", "-o", compiledBinaryPath, ".")
		output, err := cmd.CombinedOutput()
		if err != nil {
			compiledBinaryErr = &exec.ExitError{}
			compiledBinaryErr = err
			compiledBinaryPath = string(output)
		}
	})
	if compiledBinaryErr != nil {
		t.Fatalf("go build failed: %v\nOutput: %s", compiledBinaryErr, compiledBinaryPath)
	}
	return compiledBinaryPath
}

func binaryCmdTest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	command := exec.Command(compiledBinary(t), args...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func TestCompiledBinaryHelp(t *testing.T) {
	output, err := binaryCmdTest(t, "--help")
	if err != nil {
		t.Fatalf("compiled mkpod --help failed: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "Available Commands:") {
		t.Fatalf("expected help output, got: %s", output)
	}
}

func TestCompiledBinaryInitThenParseDryRun(t *testing.T) {
	target := filepath.Join(t.TempDir(), "podcast")

	if output, err := binaryCmdTest(t, "init", target); err != nil {
		t.Fatalf("compiled mkpod init failed: %v\nOutput: %s", err, output)
	}

	output, err := binaryCmdTest(t, "parse", "--spec", filepath.Join(target, "podspec.yaml"), "--dry-run")
	if err != nil {
		t.Fatalf("compiled mkpod parse --dry-run failed: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "<rss") {
		t.Fatalf("expected RSS output, got: %s", output)
	}
}

func TestCompiledBinaryLocalWorkflow(t *testing.T) {
	if _, err := exec.LookPath("lame"); err != nil {
		t.Skipf("lame unavailable: %v", err)
	}

	target := filepath.Join(t.TempDir(), "podcast")
	if output, err := binaryCmdTest(t, "init", target); err != nil {
		t.Fatalf("compiled mkpod init failed: %v\nOutput: %s", err, output)
	}

	coverPath := filepath.Join(target, "artwork", "podcast-cover.jpg")
	writeTestJPEG(t, coverPath)
	masterPath := filepath.Join(target, "masters", "episode.wav")
	writeTestWAV(t, masterPath, 44100, 1)

	specPath := filepath.Join(target, "podspec.yaml")
	specContent, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	updated := strings.Replace(string(specContent), "episodes: []", `episodes:
    - uid: 1
      title: "Episode One"
      pubDate: "Fri, 25 Mar 2022 16:00:13 +0000"
      link: "https://example.com/`+filepath.Base(target)+`/episodes/1"
      subtitle: "Integration test episode"
      description: "Generated by TestCompiledBinaryLocalWorkflow"
      input: "masters/episode.wav"
`, 1)
	if err := os.WriteFile(specPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write updated spec: %v", err)
	}

	if output, err := binaryCmdTest(t, "encode", "--spec", specPath, "1"); err != nil {
		t.Fatalf("compiled mkpod encode failed: %v\nOutput: %s", err, output)
	}

	encodedPath := filepath.Join(target, "episode.mp3")
	if _, err := os.Stat(encodedPath); err != nil {
		t.Fatalf("expected encoded output %s: %v", encodedPath, err)
	}

}

func writeTestJPEG(t *testing.T, filename string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(filename), err)
	}

	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Create(%q): %v", filename, err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 0x22, G: 0x66, B: 0xaa, A: 0xff})
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode(%q): %v", filename, err)
	}
}

func writeTestWAV(t *testing.T, filename string, sampleRate uint32, seconds uint32) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(filename), err)
	}

	const numChannels uint16 = 1
	const bitsPerSample uint16 = 16
	bytesPerSample := uint32(bitsPerSample / 8)
	numSamples := sampleRate * seconds
	dataSize := numSamples * uint32(numChannels) * bytesPerSample
	byteRate := sampleRate * uint32(numChannels) * bytesPerSample
	blockAlign := numChannels * bitsPerSample / 8
	riffSize := uint32(36) + dataSize

	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Create(%q): %v", filename, err)
	}
	defer file.Close()

	write := func(data any) {
		if err := binary.Write(file, binary.LittleEndian, data); err != nil {
			t.Fatalf("binary.Write(%q): %v", filename, err)
		}
	}

	if _, err := file.Write([]byte("RIFF")); err != nil {
		t.Fatalf("Write RIFF: %v", err)
	}
	write(riffSize)
	if _, err := file.Write([]byte("WAVEfmt ")); err != nil {
		t.Fatalf("Write WAVEfmt: %v", err)
	}
	write(uint32(16))
	write(uint16(1))
	write(numChannels)
	write(sampleRate)
	write(byteRate)
	write(blockAlign)
	write(bitsPerSample)
	if _, err := file.Write([]byte("data")); err != nil {
		t.Fatalf("Write data: %v", err)
	}
	write(dataSize)
	silence := make([]byte, dataSize)
	if _, err := file.Write(silence); err != nil {
		t.Fatalf("Write silence: %v", err)
	}
}

func TestCompiledBinaryEncodePersistsMetadataForParse(t *testing.T) {
	if _, err := exec.LookPath("lame"); err != nil {
		t.Skipf("lame unavailable: %v", err)
	}

	target := filepath.Join(t.TempDir(), "podcast")
	if output, err := binaryCmdTest(t, "init", target); err != nil {
		t.Fatalf("compiled mkpod init failed: %v\nOutput: %s", err, output)
	}

	writeTestJPEG(t, filepath.Join(target, "artwork", "podcast-cover.jpg"))
	writeTestJPEG(t, filepath.Join(target, "artwork", "episode-one.jpg"))
	writeTestWAV(t, filepath.Join(target, "masters", "episode-one.wav"), 44100, 1)

	specPath := filepath.Join(target, "podspec.yaml")
	specContent, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	updated := strings.Replace(string(specContent), "episodes: []", `episodes:
    - uid: 1
      title: "Episode One"
      pubDate: "Fri, 25 Mar 2022 16:00:13 +0000"
      link: "https://example.com/podcast/episodes/1"
      subtitle: "Integration test episode"
      description: "Generated by TestCompiledBinaryEncodePersistsMetadataForParse"
      image: "artwork/episode-one.jpg"
      input: "masters/episode-one.wav"
`, 1)
	if err := os.WriteFile(specPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write updated spec: %v", err)
	}

	if output, err := binaryCmdTest(t, "encode", "--spec", specPath, "1"); err != nil {
		t.Fatalf("compiled mkpod encode failed: %v\nOutput: %s", err, output)
	}

	specAfterEncode, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec after encode: %v", err)
	}
	specText := string(specAfterEncode)
	for _, want := range []string{"output: episode-one.mp3", "type: audio/mpeg", "length:", "duration:"} {
		if !strings.Contains(specText, want) {
			t.Fatalf("expected encoded spec to contain %q, got: %s", want, specText)
		}
	}

	if output, err := binaryCmdTest(t, "parse", "--spec", specPath); err != nil {
		t.Fatalf("compiled mkpod parse failed: %v\nOutput: %s", err, output)
	}

	rssPath := filepath.Join(target, "podcast.rss")
	rssContent, err := os.ReadFile(rssPath)
	if err != nil {
		t.Fatalf("read podcast.rss: %v", err)
	}
	rssText := string(rssContent)
	assertWellFormedRSSXMLBytes(t, rssContent)
	feed := parseCompiledBinaryRSS(t, rssContent)
	if len(feed.Channel.Items) != 1 {
		t.Fatalf("expected 1 RSS item, got %d", len(feed.Channel.Items))
	}
	if feed.Channel.Items[0].Summary == "" {
		t.Fatalf("expected RSS item summary to be present")
	}
	if feed.Channel.Items[0].Enclosure.Type != "audio/mpeg" {
		t.Fatalf("expected enclosure type audio/mpeg, got %q", feed.Channel.Items[0].Enclosure.Type)
	}
	if !strings.Contains(feed.Channel.Items[0].Enclosure.URL, "episode-one.mp3") {
		t.Fatalf("expected enclosure URL to reference episode-one.mp3, got %q", feed.Channel.Items[0].Enclosure.URL)
	}
	if !strings.Contains(feed.Channel.Items[0].Image.Href, "artwork/episode-one.jpg") {
		t.Fatalf("expected item image href to reference artwork/episode-one.jpg, got %q", feed.Channel.Items[0].Image.Href)
	}
	for _, want := range []string{
		"<title>Episode One</title>",
		"url=\"https://example-podcast-bucket.s3.us-east-1.amazonaws.com/episode-one.mp3\"",
		"type=\"audio/mpeg\"",
		"<itunes:image href=\"https://example-podcast-bucket.s3.us-east-1.amazonaws.com/artwork/episode-one.jpg\"/>",
	} {
		if !strings.Contains(rssText, want) {
			t.Fatalf("expected RSS to contain %q, got: %s", want, rssText)
		}
	}
}

func assertWellFormedRSSXMLBytes(t *testing.T, content []byte) {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(content))
	for {
		if _, err := decoder.Token(); err != nil {
			if err == io.EOF {
				return
			}
			t.Fatalf("invalid RSS XML: %v\n%s", err, string(content))
		}
	}
}

type compiledBinaryRSSFeed struct {
	Channel struct {
		Items []compiledBinaryRSSItem `xml:"item"`
	} `xml:"channel"`
}

type compiledBinaryRSSItem struct {
	Title       string `xml:"title"`
	Summary     string `xml:"summary"`
	Description string `xml:"description"`
	Enclosure   struct {
		Type string `xml:"type,attr"`
		URL  string `xml:"url,attr"`
	} `xml:"enclosure"`
	Image struct {
		Href string `xml:"href,attr"`
	} `xml:"image"`
}

func parseCompiledBinaryRSS(t *testing.T, content []byte) compiledBinaryRSSFeed {
	t.Helper()
	var feed compiledBinaryRSSFeed
	if err := xml.Unmarshal(content, &feed); err != nil {
		t.Fatalf("unmarshal RSS XML: %v\n%s", err, string(content))
	}
	return feed
}
