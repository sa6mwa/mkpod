package model

import "fmt"

type Category struct {
	Name          string   `yaml:"name"`
	Subcategories []string `yaml:"subcategories"`
}

func (c *Category) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var name string
	if err := unmarshal(&name); err == nil {
		c.Name = name
		return nil
	}

	var mapData map[string][]string
	if err := unmarshal(&mapData); err == nil {
		for k, v := range mapData {
			c.Name = k
			c.Subcategories = v
			break
		}
		return nil
	}
	// Can not use Category type as that makes unmarshalling infinitely recursive
	type CategoryInternal struct {
		Name          string   `yaml:"name"`
		Subcategories []string `yaml:"subcategories"`
	}
	var category CategoryInternal
	if err := unmarshal(&category); err != nil {
		fmt.Println(err)
		return err
	}
	c.Name = category.Name
	c.Subcategories = category.Subcategories
	return nil
}
