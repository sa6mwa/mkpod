package model

type Config struct {
	BaseURL         string    `yaml:"baseURL"`
	Image           string    `yaml:"image"`
	DefaultPodImage string    `yaml:"defaultPodImage"`
	Aws             AwsConfig `yaml:"aws"`
	LocalStorageDir string    `yaml:"localStorageDir"`
}
