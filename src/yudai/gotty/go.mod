module github.com/yudai/gotty

replace github.com/yudai/hcl => ../hcl

go 1.24

require (
	github.com/NYTimes/gziphandler v1.1.1
	github.com/creack/pty v1.1.13
	github.com/elazarl/go-bindata-assetfs v1.0.1
	github.com/fatih/structs v1.1.0
	github.com/gorilla/websocket v1.4.2
	github.com/pkg/errors v0.9.1
	github.com/urfave/cli v1.22.5
	github.com/yudai/hcl v0.0.0-00010101000000-000000000001
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.0-20190314233015-f79a8a8ca69d // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/russross/blackfriday/v2 v2.0.1 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
)
