package model

// This is only for the configuration, not implementing AWS handler logic.

type AwsConfig struct {
	Profile string  `yaml:"profile"`
	Region  string  `yaml:"region"`
	Buckets Buckets `yaml:"buckets"`
}

type Buckets struct {
	Input  string `yaml:"input"`
	Output string `yaml:"output"`
}
