package model

type ConfigInput struct {
	Needs []NeedInput `json:"needs" yaml:"needs" toml:"needs"`
}

type NeedInput struct {
	ID       string                 `json:"id" yaml:"id" toml:"id"`
	Type     TypeID                 `json:"type" yaml:"type" toml:"type"`
	Instance string                 `json:"instance" yaml:"instance" toml:"instance"`
	Dotenv   *DotenvProjectionInput `json:"dotenv,omitempty" yaml:"dotenv,omitempty" toml:"dotenv,omitempty"`
}

type DotenvProjectionInput struct {
	Fields []DotenvFieldBindingInput `json:"fields,omitempty" yaml:"fields,omitempty" toml:"fields,omitempty"`
}

type DotenvFieldBindingInput struct {
	Field string `json:"field" yaml:"field" toml:"field"`
	Key   string `json:"key" yaml:"key" toml:"key"`
}
