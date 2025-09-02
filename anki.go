package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/dstotijn/go-notion"
)

var ErrAnkiConnectFailed = errors.New("anki: could not connect to AnkiConnect")

type Anki struct {
	Config AnkiConfig
}

type AnkiConfig struct {
	AnkiConnectURL string `json:"anki_connect_url"`
	DeckName       string `json:"deck_name"`
	ModelName      string `json:"model_name"`
	httpClient     *http.Client
}

type AnkiConnectRequest struct {
	Action  string      `json:"action"`
	Version int         `json:"version"`
	Params  interface{} `json:"params"`
}

type AnkiConnectResponse struct {
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

type AnkiNote struct {
	DeckName  string            `json:"deckName"`
	ModelName string            `json:"modelName"`
	Fields    map[string]string `json:"fields"`
	Tags      []string          `json:"tags"`
}

type AddNotesParams struct {
	Notes []AnkiNote `json:"notes"`
}

func NewAnki(url, deckName, modelName string) *Anki {
	return &Anki{
		Config: AnkiConfig{
			AnkiConnectURL: url,
			DeckName:       deckName,
			ModelName:      modelName,
			httpClient:     &http.Client{Timeout: 30 * time.Second},
		},
	}
}

func (anki *Anki) makeJSONRequest(payload interface{}, result interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("fail to serialize request: %v", err)
	}

	resp, err := anki.Config.httpClient.Post(anki.Config.AnkiConnectURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("fail to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fail to receive valid response: %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

func (anki *Anki) CheckAnkiConnect() error {
	request := AnkiConnectRequest{
		Action:  "version",
		Version: 6,
		Params:  map[string]interface{}{},
	}

	var response AnkiConnectResponse
	err := anki.makeJSONRequest(request, &response)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAnkiConnectFailed, err)
	}

	if response.Error != nil {
		return fmt.Errorf("AnkiConnect check error: %v", response.Error)
	}

	return nil
}

func (anki *Anki) CreateDeck(deckName string) error {
	if deckName == "" {
		return fmt.Errorf("no deck name provided")
	}

	request := AnkiConnectRequest{
		Action: "createDeck",
		Params: map[string]interface{}{
			"deck": deckName,
		},
		Version: 6,
	}

	var response AnkiConnectResponse
	err := anki.makeJSONRequest(request, &response)
	if err != nil {
		return fmt.Errorf("fail to create deck: %v", err)
	}

	if response.Error != nil {
		return fmt.Errorf("AnkiConnect deck creation error: %v", response.Error)
	}
	log.Printf("Successfully created deck: %s", deckName)

	return nil
}

func (anki *Anki) EnsureDeckExists() error {
	deckName := anki.Config.DeckName
	if deckName != "" {
		request := AnkiConnectRequest{
			Action:  "deckNames",
			Params:  map[string]interface{}{},
			Version: 6,
		}

		var response AnkiConnectResponse
		err := anki.makeJSONRequest(request, &response)
		if err != nil {
			return fmt.Errorf("fail to check existing decks: %v", err)
		}

		if response.Error != nil {
			return fmt.Errorf("AnkiConnect deck exist error: %v", response.Error)
		}

		deckNames, ok := response.Result.([]interface{})
		if !ok {
			return fmt.Errorf("invalid response format: %v", response.Result)
		}

		for _, name := range deckNames {
			if name == deckName {
				log.Printf("Deck already exists: %s", deckName)
				return nil
			}
		}

		log.Printf("Deck does not exist, creating: %s", deckName)
		return anki.CreateDeck(deckName)
	}

	return fmt.Errorf("no deck name provided")
}

func (anki *Anki) EnsureModelExists(pageProperties notion.DatabasePageProperties) error {
	configModelName := anki.Config.ModelName
	request := AnkiConnectRequest{
		Action:  "modelNames",
		Params:  map[string]interface{}{},
		Version: 6,
	}

	var response AnkiConnectResponse
	err := anki.makeJSONRequest(request, &response)
	if err != nil {
		return fmt.Errorf("fail to check existing models: %v", err)
	}

	if response.Error != nil {
		return fmt.Errorf("AnkiConnect model check error: %v", response.Error)
	}

	modelNames, ok := response.Result.([]interface{})
	if !ok {
		return fmt.Errorf("invalid response format: %v", response.Result)
	}

	for _, name := range modelNames {
		if name == configModelName {
			log.Printf("Model already exists: %s", configModelName)
			return nil
		}
	}

	log.Printf("Model does not exist, creating: %s", configModelName)
	fields := []string{}
	for name := range pageProperties {
		fields = append(fields, name)
	}
	return anki.createModel(configModelName, fields)
}

func (anki *Anki) createModel(modelName string, fields []string) error {
	if modelName == "" {
		return fmt.Errorf("no model name provided")
	}
	request := AnkiConnectRequest{
		Action:  "createModel",
		Version: 6,
		Params: map[string]any{
			"modelName":     modelName,
			"inOrderFields": fields,
			"cardTemplates": []map[string]any{
				{
					"Name":  "Card 2",
					"Front": "{{}}",
					"Back":  "{{}}",
				},
			},
		},
	}

	var response AnkiConnectResponse
	err := anki.makeJSONRequest(request, &response)
	if err != nil {
		return fmt.Errorf("fail to create model: %v", err)
	}

	if response.Error != nil {
		return fmt.Errorf("AnkiConnect model creation error: %v", response.Error)
	}

	log.Printf("Successfully created model: %s", modelName)
	return nil
}

func (anki *Anki) AddNotesToDeck(fields []map[string]string) error {
	var ankiNotes []AnkiNote
	for _, noteFields := range fields {
		ankiNotes = append(ankiNotes, AnkiNote{
			DeckName:  anki.Config.DeckName,
			ModelName: anki.Config.ModelName,
			Fields:    noteFields,
			Tags:      []string{"notion"},
		})
	}

	request := AnkiConnectRequest{
		Action:  "addNotes",
		Version: 6,
		Params: AddNotesParams{
			Notes: ankiNotes,
		},
	}

	var response AnkiConnectResponse
	err := anki.makeJSONRequest(request, &response)
	if err != nil {
		log.Printf("fail to add note: %v", err)
	}

	if response.Error != nil {
		log.Printf("AnkiConnect error: %v", response.Error)
	}

	log.Printf("Successfully added note to deck: %s", anki.Config.DeckName)
	return nil
}

func (anki *Anki) CanAddNotes(fields map[string]string) (bool, error) {
	request := AnkiConnectRequest{
		Action:  "canAddNotes",
		Version: 6,
		Params: AddNotesParams{
			Notes: []AnkiNote{{
				DeckName:  anki.Config.DeckName,
				ModelName: anki.Config.ModelName,
				Fields:    fields,
				Tags:      []string{"notion"},
			}},
		},
	}

	var response AnkiConnectResponse
	err := anki.makeJSONRequest(request, &response)
	if err != nil {
		return false, fmt.Errorf("fail to fetch notes by deck: %v", err)
	}

	if response.Error != nil {
		return false, fmt.Errorf("AnkiConnect fetch notes error: %v", response.Error)
	}

	result, ok := response.Result.([]interface{})
	if !ok {
		return false, fmt.Errorf("invalid response format: %v", response.Result)
	}

	var indicators []bool
	for _, indicator := range result {
		if indicator == true {
			indicators = append(indicators, true)
		} else {
			indicators = append(indicators, false)
		}
	}

	if len(indicators) == 0 {
		return false, fmt.Errorf("no indicators returned")
	}

	return indicators[0], nil
}
