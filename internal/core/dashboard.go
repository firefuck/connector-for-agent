package core

import (
	"bytes"
	_ "embed"
	"html/template"
	"relaydesk"
)

//go:embed web/dashboard.html
var dashboardTemplate string

var dashboard = func() string {
	t := template.Must(template.New("dashboard").Parse(dashboardTemplate))
	var page bytes.Buffer
	if err := t.Execute(&page, struct{ Name, Series string }{relaydesk.Name, relaydesk.Series}); err != nil {
		panic(err)
	}
	return page.String()
}()

//go:embed web/dashboard.css
var dashboardCSS string

//go:embed web/dashboard.js
var dashboardJS string
