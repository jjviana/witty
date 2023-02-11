package config

import (
	"encoding/json"
	"errors"
	"os"
)

// Repository is a configuration repository. It stores and loads configuration
// in a configurable base directory.
type Repository struct {
	directory string
}

// NewRepository creates a new configuration repository.
func NewRepository(directory string) *Repository {
	return &Repository{directory: directory}
}

// Store stores the provided configuration object in the repository, with the given name,
// The object supplied must be json-serializable.
func (r *Repository) Store(name string, config interface{}) error {
	// We serialize the object as a json file named 'name.json' in the repository
	// directory.

	makeDirIfNotExists(r.directory)

	file, err := os.OpenFile(r.directory+"/"+name+".json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(config)
	if err != nil {
		return err
	}

	return nil

}

// Load loads the configuration object with the given name from the repository.
// The object supplied must be json-deserializable.
func (r *Repository) Load(name string, config interface{}) error {
	// We deserialize the object from a json file named 'name.json' in the repository
	// directory.
	file, err := os.Open(r.directory + "/" + name + ".json")
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		return err
	}

	return nil
}

func makeDirIfNotExists(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		err = os.Mkdir(dir, 0700)
		if err != nil {
			return err
		}
	} else {
		if !info.IsDir() {
			return errors.New("Cannot create directory " + dir)
		}
	}
	return nil
}
