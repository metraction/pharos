package model

type EnricherType string

const (
	EnricherTypeVisual   EnricherType = "visual"
	EnricherTypeYaegi    EnricherType = "yaegi"
	EnricherTypeStarlark EnricherType = "starlark"
	EnricherTypeHbs      EnricherType = "hbs"
)

type Enricher struct {
	ID          uint         `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string       `json:"name"`
	Type        EnricherType `json:"type" enum:"visual,yaegi,starlark,hbs"`
	Description string       `json:"description"`
	Enabled     bool         `json:"enabled"`
	Code        string       `json:"code"`
}
