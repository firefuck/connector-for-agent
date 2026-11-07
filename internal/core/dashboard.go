package core

import _ "embed"

//go:embed web/dashboard.html
var dashboard string

//go:embed web/dashboard.css
var dashboardCSS string

//go:embed web/dashboard.js
var dashboardJS string
