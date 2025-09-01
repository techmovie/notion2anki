# Notion2Anki

🚀 An automated tool that syncs content from Notion databases to Anki flashcards.

## ✨ Features

- 🔄 **Auto Sync**: Automatically sync updated content from Notion database to Anki
- 📚 **Smart Filtering**: Only sync recently edited content to avoid duplicates
- 🔧 **Auto Configuration**: Automatically create Anki note types and decks
- ⚡ **Real-time Monitoring**: Continuously monitor Notion database changes
- 🧩 **Modular Processors**: Extensible processor system for custom note processing
- 🔐 **Secure Configuration**: Support for 1Password CLI integration for secure local token management

## 🛠️ Installation

### Prerequisites

- [Anki](https://apps.ankiweb.net/) Desktop
- [AnkiConnect](https://github.com/FooSoft/anki-connect) plugin
- Go 1.21 or later (for building from source)

### Clone the project

```bash
git clone https://github.com/notion2anki/notion2anki.git
cd notion2anki
```

### Install dependencies

```bash
go mod download
```

### Build

```bash
go build -o notion2anki
```

## ⚙️ Configuration

### 1. Install AnkiConnect

Install the [AnkiConnect](https://ankiweb.net/shared/info/2055492159) plugin in Anki and restart Anki.

### 2. Configure using config.yaml

The application now uses a YAML configuration file instead of environment variables. Copy the example configuration:

```bash
cp config.yaml.example config.yaml
```

Edit `config.yaml` with your settings:

```yaml
anki:
  deck_name: "anki_deck_name"
  model_name: "anki_model_name"
  connect_url: "http://localhost:8765"

notion:
  token: "your_notion_token_here"  # or use 1Password: "op://<vault-name>/<item-name>/[section-name/]<field-name>"
  database_id: "your_database_id"
  poll_interval_seconds: 300
  primary_key_field: "Word"

processors:
  - name: "dwds_audio"
    target_field: "Audio"
    source_field: "Word"
    enabled: true
    config: {}
```

### 3. Get Notion Token

1. Visit [Notion Integrations](https://www.notion.so/my-integrations)
2. Click "New integration"
3. Fill in basic information and create
4. Copy the "Internal Integration Token"

### 4. Get Database ID

1. Open your Notion database page
2. Copy the database ID from the URL (32-character string)
3. Make sure to share the database with the Integration you created

### 5. Notion Database Structure

You are free to customize the database structure as needed. Here is a structure for German language learning:

| Property | Type | Description |
|----------|------|-------------|
| Word | Title | German word |
| Translation | Rich Text | Chinese/English translation |
| Example | Rich Text | Example sentence |
| Artikel | Select | (der/die/das) |
| Plural | Rich Text | Plural form |
| IPA | Rich Text | Phonetic transcription |
| Tag | Multi-select | Category tags |
| Audio | Rich Text | Audio file URL (auto-filled) |

## 🚀 Usage

### Start syncing

```bash
./notion2anki
```

Or run directly with Go:

```bash
go run .
```

## 🐳 Docker Usage

### Using Docker Compose (Recommended)

```bash
# Basic usage
docker compose up -d

# View logs
docker compose logs -f notion2anki

# Stop the service
docker compose down
```



### Manual Docker Build

```bash
# Build the image
docker build -t notion2anki .

# Run the container
docker run -d \
  --name notion2anki \
  --network host \
  -v ./config.yaml:/app/config.yaml:ro \
  notion2anki
```


## 🔧 How It Works

1. **Initialization**: Program starts by loading configuration and connecting to Notion API and AnkiConnect
2. **Query Updates**: Periodically query pages from Notion database
3. **Data Processing**: Extract page properties and format them for Anki cards
4. **Processor Pipeline**: Run configured processors (e.g., DWDS audio fetcher) on note data
5. **Create Cards**: Check for duplicates and add new cards to specified deck
6. **Continuous Monitoring**: Repeat the above process based on configured interval

## 🧩 Processor System

The application features a modular processor system that allows you to extend functionality:

### Built-in Processors

- **dwds_audio**: Automatically fetches German word pronunciations from DWDS dictionary

### Processor Configuration

Configure processors in `config.yaml`:

```yaml
processors:
  - name: "dwds_audio"
    target_field: "Audio"      # Field to store the audio URL
    source_field: "Word"       # Field to read the German word from
    enabled: true              # Enable/disable this processor
    config: {}                 # Processor-specific configuration
```

### Creating Custom Processors

1. Implement the `NoteProcessor` interface in the `processors` package
2. Register your processor in the `main.go` init function
3. Configure it in your `config.yaml`

## 🎯 Configuration Options

### Sync Interval

Configure the sync interval in `config.yaml`:

```yaml
notion:
  poll_interval_seconds: 300  # Check for updates every 5 minutes
```

### Note Type

The program automatically creates a note type with fields matching your Notion database properties.


## 🤝 Contributing

Issues and Pull Requests are welcome!


## 🙏 Acknowledgments

- [go-notion](https://github.com/dstotijn/go-notion) - Notion API Go client
- [AnkiConnect](https://github.com/FooSoft/anki-connect) - Anki Desktop API
- [DWDS](https://www.dwds.de/) - German dictionary and audio source
- [goquery](https://github.com/PuerkitoBio/goquery) - HTML parsing library
- [viper](https://github.com/spf13/viper) - Configuration management
- [resty](https://github.com/go-resty/resty) - HTTP client library

## 📈 Roadmap

- [ ] Support for multiple Notion databases
- [ ] Real-time sync using Notion webhooks

---

If this project helps you, please give it a ⭐ star!
