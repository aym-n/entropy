package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
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

	var client *genai.Client
	if config.Gpt.Enabled {
		client = getGenAIClient(config.Gpt.ApiKey)
	}

	log.Println("Watching 'magic' folder...")

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Create == fsnotify.Create {
				time.Sleep(500 * time.Millisecond)
				log.Println("New file detected:", event.Name)

				targetFolder := matchRules(event.Name, config.Rules)

				if targetFolder == "" && config.Gpt.Enabled {
					targetFolder = suggestFolderWithGenAI(context.Background(), client, config.Gpt.Model, config.Gpt.Instructions, event.Name)
					log.Println("AI suggested folder:", targetFolder)
				}

				if targetFolder == "" {
					targetFolder = "Unsorted"
				}

				organizeFile(event.Name, targetFolder)
			}

		case err := <-watcher.Errors:
			log.Println("Watcher error:", err)
		}
	}

}
