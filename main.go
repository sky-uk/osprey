package main

import (
	"net/http"

	"github.com/sky-uk/osprey/v2/cmd"
	"github.com/sky-uk/osprey/v2/common/web"
)

func main() {
	http.DefaultTransport = web.DefaultTransport()

	cmd.Execute()
}
