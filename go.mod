module github.com/blocktree/nulsio-adapter

go 1.12

require (
	github.com/asdine/storm v2.1.2+incompatible
	github.com/astaxie/beego v1.11.1
	github.com/blocktree/go-owcdrivers v1.0.12
	github.com/blocktree/go-owcrypt v1.0.1
	github.com/blocktree/openwallet v1.4.6
	github.com/imroc/req v0.2.3
	github.com/pkg/errors v0.8.1
	github.com/shopspring/decimal v0.0.0-20180709203117-cd690d0c9e24
	github.com/tidwall/gjson v1.2.1
	golang.org/x/crypto v0.0.0-20190404164418-38d8ce5564a5
)

//replace github.com/blocktree/openwallet => ../../openwallet
