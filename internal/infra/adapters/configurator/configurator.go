// configurator is an adapter for loading and saving the main
// aggregate constituting the podcast as well as setting and getting
// runtime properties globally accessible. It implements the
// ports.ForConfiguring interface.
package configurator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/app/ports"
	"gopkg.in/yaml.v3"
)

var DefaultSpecfile string = "podspec.yaml"

// configurator.New returns a local file-based configurator that
// satisfies the ports.ForConfiguring port interface.
func New(podcastYamlFilename string) ports.ForConfiguring {
	if podcastYamlFilename == "" {
		podcastYamlFilename = DefaultSpecfile
	}
	return &forConfiguring{
		properties: make(map[string]any),
		specFile:   podcastYamlFilename,
	}
}

// Implements the ports.ForConfiguring interface.
type forConfiguring struct {
	properties map[string]any
	specFile   string
}

func (c *forConfiguring) Get(ctx context.Context, property string) (any, error) {
	v, ok := c.properties[property]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (c *forConfiguring) Set(ctx context.Context, property string, value any) error {
	if c.properties == nil {
		c.properties = make(map[string]any)
	}
	c.properties[property] = value
	return nil
}

func (c *forConfiguring) Load(ctx context.Context) (*model.Atom, error) {
	f, err := os.Open(c.specFile)
	if err != nil {
		return nil, err
	}
	var atom model.Atom
	if err := yaml.NewDecoder(f).Decode(&atom); err != nil {
		return nil, err
	}
	
	// Validate required top-level fields
	if err := c.validateRequiredFields(&atom); err != nil {
		return nil, fmt.Errorf("validation failed for %s: %w", c.specFile, err)
	}
	
	// Set defaults
	if atom.Encoding.CRF == 0 {
		atom.Encoding.CRF = 28
	}
	if strings.TrimSpace(atom.Encoding.ABR) == "" {
		atom.Encoding.ABR = "128k"
	}
	// Set pubDate to current time if it's zero (missing or zero value)
	if atom.PubDate.IsZero() {
		atom.PubDate.Time = time.Now().UTC()
	}
	return &atom, nil
}

func (c *forConfiguring) Save(ctx context.Context, atom *model.Atom) error {
	atom.LastBuildDate.Time = time.Now().UTC()
	f, err := os.Create(c.specFile)
	if err != nil {
		return fmt.Errorf("unable to re-write %s: %w", c.specFile, err)
	}
	defer f.Close()
	if err := yaml.NewEncoder(f).Encode(atom); err != nil {
		return fmt.Errorf("unable to marshall yaml: %w", err)
	}
	return nil
}

// validateRequiredFields validates that all required top-level fields are present and not empty
func (c *forConfiguring) validateRequiredFields(atom *model.Atom) error {
	var missingFields []string
	
	// Check required string fields
	if strings.TrimSpace(atom.Author) == "" {
		missingFields = append(missingFields, "author")
	}
	if strings.TrimSpace(atom.Config.BaseURL) == "" {
		missingFields = append(missingFields, "config.baseURL")
	}
	if strings.TrimSpace(atom.Config.Image) == "" {
		missingFields = append(missingFields, "config.image")
	}
	if strings.TrimSpace(atom.Config.DefaultPodImage) == "" {
		missingFields = append(missingFields, "config.defaultPodImage")
	}
	if strings.TrimSpace(atom.Atom) == "" {
		missingFields = append(missingFields, "atom")
	}
	if strings.TrimSpace(atom.Title) == "" {
		missingFields = append(missingFields, "title")
	}
	if atom.TTL == 0 {
		missingFields = append(missingFields, "ttl")
	}
	if strings.TrimSpace(atom.Language) == "" {
		missingFields = append(missingFields, "language")
	}
	if strings.TrimSpace(atom.Copyright) == "" {
		missingFields = append(missingFields, "copyright")
	}
	if strings.TrimSpace(atom.WebMaster) == "" {
		missingFields = append(missingFields, "webMaster")
	}
	if strings.TrimSpace(atom.Description) == "" {
		missingFields = append(missingFields, "description")
	}
	if strings.TrimSpace(atom.Subtitle) == "" {
		missingFields = append(missingFields, "subtitle")
	}
	if strings.TrimSpace(atom.OwnerName) == "" {
		missingFields = append(missingFields, "ownerName")
	}
	if strings.TrimSpace(atom.OwnerEmail) == "" {
		missingFields = append(missingFields, "ownerEmail")
	}
	
	// Note: pubDate is not validated here as it's defaulted to current time in Load() if missing
	
	if len(missingFields) > 0 {
		return fmt.Errorf("required fields are missing or empty: %s", strings.Join(missingFields, ", "))
	}
	
	return nil
}
