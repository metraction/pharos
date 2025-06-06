package globals

import "github.com/metraction/pharos/models"

// We all hate global variables but some should be global, at least we have them all in one place here.

// Config holds the configuration, this is a global variable so we don't have to pass it around
var Config *models.Config
