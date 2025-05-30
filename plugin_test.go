package main

import (
	"github.com/fghwett/gotify-plugin-forward/app"
	"testing"

	"github.com/gotify/plugin-api"
	"github.com/stretchr/testify/assert"
)

func TestAPICompatibility(t *testing.T) {
	assert.Implements(t, (*plugin.Plugin)(nil), &app.App{})
	// Add other interfaces you intend to implement here
}
