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

var defaultSpecfile string = "podspec.yaml"

// configurator.New returns a local file-based configurator that
// satisfies the ports.ForConfiguring port interface.
func New(podcastYamlFilename string) ports.ForConfiguring {
	if podcastYamlFilename == "" {
		podcastYamlFilename = defaultSpecfile
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
	// Set defaults
	if atom.Encoding.CRF == 0 {
		atom.Encoding.CRF = 28
	}
	if strings.TrimSpace(atom.Encoding.ABR) == "" {
		atom.Encoding.ABR = "128k"
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
