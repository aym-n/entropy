package main

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

type Rule struct {
	Pattern string `yaml:"pattern"`
	Target  string `yaml:"target"`
}

type GptConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Instructions string `yaml:"instructions"`
}

type Config struct {
	Rules []Rule    `yaml:"rules"`
	Gpt   GptConfig `yaml:"gpt"`
}

func loadConfig(path string) Config {
	// TODO: have a config file , rules and knowlege / custom instructions
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("couldn't open file %s: %v", path, err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Invalid YAML: %v", err)
	}

	return config
}

func matchRules(filename string, rules []Rule) string {
	for _, rule := range rules {
		re := regexp.MustCompile(rule.Pattern)
		if re.MatchString(filename) {
			return rule.Target
		}
	}
	return ""
}

func organizeFile(filePath string, targetFolder string) {
	filename := filepath.Base(filePath)

	// TODO: add auto rename functionality
	destDir := filepath.Join("organized", targetFolder)
	os.MkdirAll(destDir, os.ModePerm) // create target folder if it doesn't exist
	destPath := filepath.Join(destDir, filename)

	err := os.Rename(filePath, destPath)
	if err != nil {
		log.Printf("Failed to move %s: %v", filename, err)
		return
	}

	log.Printf("Moved %s → %s", filename, destDir)
}

func main() {
	os.MkdirAll("magic", os.ModePerm)
	os.MkdirAll("organized", os.ModePerm)

	config := loadConfig("rules.yaml")
	log.Println("Loaded rules:", config.Rules)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	defer watcher.Close()
	err = watcher.Add("magic")

	if err != nil {
		log.Fatal(err)
	}

	log.Println("Watching 'magic' folder...")

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Create == fsnotify.Create {
				time.Sleep(500 * time.Millisecond)
				log.Println("New file detected:", event.Name)

				target := matchRules(event.Name, config.Rules)
				if target != "" {
					organizeFile(event.Name, target)
				} else {
					log.Println("No matching rule for:", event.Name)
					// TODO: Add GPT fallback
				}
			}

		case err := <-watcher.Errors:
			log.Println("Watcher error:", err)
		}
	}

}
