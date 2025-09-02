package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/notion2anki/processors"
	"github.com/spf13/viper"
)

type Config struct {
	AnkiConnectURL   string
	DeckName         string
	ModelName        string
	NotionToken      string
	NotionDatabaseID string
	PollInterval     time.Duration
	Processors       []processors.ProcessorConfig
}

var processorRegistry = make(map[string]processors.NoteProcessor)

func registerProcessor(p processors.NoteProcessor) {
	processorRegistry[p.Name()] = p
}
func loadConfig() (*Config, error) {

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatal("Config file not found")
		} else {
			log.Fatalf("Error reading config file: %v", err)
		}
	}
	pollInterval := viper.GetInt("notion.poll_interval_seconds")

	if pollInterval < 0 {
		return nil, fmt.Errorf("invalid notion.poll_interval_seconds: %d", pollInterval)
	}

	var processorConfigs []processors.ProcessorConfig
	if err := viper.UnmarshalKey("processors", &processorConfigs); err != nil {
		return nil, fmt.Errorf("failed to parse processors config: %v", err)
	}

	return &Config{
		AnkiConnectURL:   viper.GetString("anki.connect_url"),
		DeckName:         viper.GetString("anki.deck_name"),
		ModelName:        viper.GetString("anki.model_name"),
		NotionToken:      viper.GetString("notion.token"),
		NotionDatabaseID: viper.GetString("notion.database_id"),
		PollInterval:     time.Duration(pollInterval),
		Processors:       processorConfigs,
	}, nil
}

func sync(anki *Anki, nt *NotionClient, cfg *Config) error {
	log.Println("🚀 Start syncing...")
	ctx := context.Background()

	if err := anki.CheckAnkiConnect(); err != nil {
		return err
	}

	pages, pageProperties, err := nt.QueryAllPages(ctx)
	if err != nil {
		return err
	}

	if err := anki.EnsureDeckExists(); err != nil {
		return err
	}

	if err := anki.EnsureModelExists(pageProperties); err != nil {
		return err
	}

	notesToAdd := []map[string]string{}

	for _, page := range pages {
		properties := nt.ExtractPropertiesFromPage(page)

		canBeAdded, err := anki.CanAddNotes(properties)

		if err != nil {
			log.Printf("Error checking if note can be added: %v", err)
			continue
		}

		if !canBeAdded {
			log.Printf("Note cannot be added: %v", properties)
			continue
		}
		for _, processConfig := range cfg.Processors {
			if !processConfig.Enabled {
				continue
			}
			processor, exist := processorRegistry[processConfig.Name]
			if !exist {
				log.Printf("Processor %s not found in registry, skipping", processConfig.Name)
			}
			if err := processor.Process(&properties, processConfig); err != nil {
				log.Printf("Error from processor %s: %v", processConfig.Name, err)
			}
			if err := nt.UpdatePageOfDatabase(page, map[string]string{
				processConfig.TargetField: properties[processConfig.TargetField],
			}, pageProperties); err != nil {
				log.Printf("Failed to update Notion page %s: %v", page.ID, err)
			}
		}
		notesToAdd = append(notesToAdd, properties)
	}

	if len(notesToAdd) > 0 {
		log.Printf("Adding %d new notes to Anki...", len(notesToAdd))
		if err := anki.AddNotesToDeck(notesToAdd); err != nil {
			log.Printf("Failed to add notes to Anki: %v", err)
		}
	} else {
		log.Println("No new notes to add.")
	}

	nt.LastSyncTime = time.Now()
	log.Println("Sync completed.")
	return nil
}
func isFatalError(err error) bool {
	if errors.Is(err, ErrAnkiConnectFailed) || errors.Is(err, ErrNotionAuthFailed) || errors.Is(err, ErrNotionDBNotFound) {
		return true
	}
	return false
}

func Start(anki *Anki, nt *NotionClient, cfg *Config) {

	log.Printf("start: %d seconds", nt.PollInterval)

	if err := sync(anki, nt, cfg); err != nil {
		if isFatalError(err) {
			log.Fatalf("Fatal error during initial sync, shutting down: %v", err)
		}
		log.Printf("fail to sync: %v", err)
	}

	ticker := time.NewTicker(time.Duration(nt.PollInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := sync(anki, nt, cfg); err != nil {
			log.Printf("fail to sync: %v", err)
		}
	}
}

func init() {
	registerProcessor(processors.NewDWDSAudioProcessor())
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
	anki := NewAnki(cfg.AnkiConnectURL, cfg.DeckName, cfg.ModelName)

	nt := NewNotion(cfg.NotionToken, cfg.NotionDatabaseID, cfg.PollInterval)

	Start(anki, nt, cfg)

}
