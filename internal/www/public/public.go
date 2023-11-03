package public

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
)

//go:embed scripts/*.js styles/*.css
var Content embed.FS

var ErrNoStyles = errors.New("no styles found")

func MustGetStyles() string {
	styles, err := getStyles()
	if err != nil {
		panic(err)
	}

	return styles
}

func MustGetScripts() string {
	scripts, err := getScripts()
	if err != nil {
		panic(err)
	}

	return scripts
}

func getStyles() (string, error) {
	styles, err := fs.Glob(Content, "styles/*.css")
	if err != nil {
		return "", fmt.Errorf("failed to get styles: %w", err)
	}

	if len(styles) == 0 {
		return "", ErrNoStyles
	}

	return "/public/" + styles[0], nil
}

func getScripts() (string, error) {
	scripts, err := fs.Glob(Content, "scripts/*.js")
	if err != nil {
		return "", fmt.Errorf("failed to get scripts: %w", err)
	}

	return "/public/" + scripts[0], nil
}
