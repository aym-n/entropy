package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"google.golang.org/genai"
	"gopkg.in/yaml.v3"
)

type Rule struct {
	Pattern string `yaml:"pattern"`
	Target  string `yaml:"target"`
}

type GptConfig struct {
	Enabled      bool   `yaml:"enabled"`
	ApiKey       string `yaml:"api_key"`
	Model        string `yaml:"model"`
	Instructions string `yaml:"instructions"`
}

type Config struct {
	Rules []Rule    `yaml:"rules"`
	Gpt   GptConfig `yaml:"gpt"`
}

func isIgnored(name string) bool {
	ignoredFiles := []string{".DS_Store", "Thumbs.db"} // TODO: have OS wise ignored files as well as have config option to add more
	base := filepath.Base(name)
	if strings.HasPrefix(base, "._") {
		return true
	}
	for _, ign := range ignoredFiles {
		if base == ign {
			return true
		}
	}
	return false
}

func getGenAIClient(apiKey string) *genai.Client {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatalf("Failed to create GenAI client: %v", err)
	}
	return client
}

func suggestFolderWithGenAI(ctx context.Context, client *genai.Client, modelName, instructions, filename string) string {
	// TODO: add file metadata and content as context to the model
	// TODO: add context about folder structure
	prompt := fmt.Sprintf("%s\nFilename: %s", instructions, filename)
	resp, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), nil)
	if err != nil {
		log.Println("GenAI error:", err)
		return "Unsorted"
	}
	text := resp.Text()
	if text == "" {
		return "Unsorted"
	}
	return text
}

func loadConfig(path string) Config {
	// TODO: have a config file , rules and knowledge / custom instructions
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

func organizeItem(srcPath, targetFolder string) {
	base := filepath.Base(srcPath)
	destDir := filepath.Join("entropy", targetFolder)

	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		log.Printf("Failed to create dir %s: %v", destDir, err)
		return
	}

	destPath := filepath.Join(destDir, base)
	if err := os.Rename(srcPath, destPath); err != nil {
		log.Printf("Failed to move %s: %v", base, err)
		return
	}

	log.Printf("Moved %s → %s", base, destDir)
}

func main() {
	os.MkdirAll("entropy", os.ModePerm)

	config := loadConfig("rules.yaml")
	log.Println("Loaded rules:", config.Rules)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	defer watcher.Close()
	err = watcher.Add("entropy")

	if err != nil {
		log.Fatal(err)
	}

	var client *genai.Client
	if config.Gpt.Enabled {
		client = getGenAIClient(config.Gpt.ApiKey)
	}

	log.Println("Watching 'entropy' folder...")

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Create == fsnotify.Create {

				// skips directories
				fi, err := os.Stat(event.Name)
				if err == nil && fi.IsDir() {
					// TODO: handle directories as a single unit , you can add some config file int the folder to handle how that folder should be treated
					continue
				}

				if filepath.Dir(event.Name) != "entropy" {
					continue
				}

				time.Sleep(500 * time.Millisecond)
				log.Println("New file detected:", event.Name)

				name := filepath.Base(event.Name)

				if isIgnored(name) {
					log.Println("Ignored system file:", name)
					continue
				}
				targetFolder := matchRules(name, config.Rules)

				if targetFolder == "" && config.Gpt.Enabled {
					targetFolder = suggestFolderWithGenAI(context.Background(), client, config.Gpt.Model, config.Gpt.Instructions, event.Name)
					log.Println("AI suggested folder:", targetFolder)
				}

				if targetFolder == "" {
					targetFolder = "Unsorted"
				}

				targetFolder = strings.TrimSpace(targetFolder)
				organizeItem(event.Name, targetFolder)
			}

		case err := <-watcher.Errors:
			log.Println("Watcher error:", err)
		}
	}

}
