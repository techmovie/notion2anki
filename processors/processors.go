package processors

type ProcessorConfig struct {
	Name        string                 `mapstructure:"name"`
	Enabled     bool                   `mapstructure:"enabled"`
	TargetField string                 `mapstructure:"target_field"`
	SourceField string                 `mapstructure:"source_field"`
	Config      map[string]interface{} `mapstructure:"config"`
}

type NoteProcessor interface {
	Name() string
	Process(noteData *map[string]string, config ProcessorConfig) error
}
