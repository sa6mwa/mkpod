package model

import "strings"

// This is only for the configuration, not implementing AWS handler logic.

type AwsConfig struct {
	Profile string  `yaml:"profile"`
	Region  string  `yaml:"region"`
	Buckets Buckets `yaml:"buckets"`
}

type Buckets struct {
	Input              string `yaml:"input"`
	InputStorageClass  string `yaml:"inputStorageClass,omitempty"`
	Output             string `yaml:"output"`
	OutputStorageClass string `yaml:"outputStorageClass,omitempty"`
}

func (b *Buckets) GetStorageClass(bucket string) string {
	defaultStorageClass := "STANDARD"
	if b.InputStorageClass == "" {
		b.InputStorageClass = defaultStorageClass
	}
	if b.OutputStorageClass == "" {
		b.OutputStorageClass = defaultStorageClass
	}
	switch {
	case strings.EqualFold(b.Input, bucket):
		return b.InputStorageClass
	case strings.EqualFold(b.Output, bucket):
		return b.OutputStorageClass
	}
	return defaultStorageClass
}
