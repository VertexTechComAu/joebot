module github.com/harmonicinc-com/joebot

replace (
	github.com/filebrowser/filebrowser/v2 => ../filebrowser
	github.com/ginuerzh/gost => ../ginuerzh/gost
	github.com/yudai/gotty => ../yudai/gotty
	github.com/yudai/hcl => ../yudai/hcl
)

go 1.23

require (
	github.com/asdine/storm v2.1.2+incompatible
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/filebrowser/filebrowser/v2 v2.0.0-00010101000000-000000000000
	github.com/ginuerzh/gost v0.0.0-00010101000000-000000000000
	github.com/hashicorp/yamux v0.0.0-20210316155119-a95892c5f864
	github.com/jmoiron/sqlx v1.4.0
	github.com/labstack/echo v3.3.10+incompatible
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.13.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/afero v1.2.2
	github.com/twinj/uuid v1.0.0
	github.com/yudai/gotty v0.0.0-00010101000000-000000000000
	golang.org/x/crypto v0.28.0
	golang.org/x/net v0.30.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	modernc.org/sqlite v1.34.1
)

require (
	git.torproject.org/pluggable-transports/goptlib.git v0.0.0-20180321061416-7d56ec4f381e // indirect
	git.torproject.org/pluggable-transports/obfs4.git v0.0.0-20181103133120-08f4d470188e // indirect
	github.com/DataDog/zstd v1.4.8 // indirect
	github.com/LiamHaworth/go-tproxy v0.0.0-20190726054950-ef7efd7f24ed // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/Sereal/Sereal v0.0.0-20200820125258-a016b7cda3f3 // indirect
	github.com/Yawning/chacha20 v0.0.0-20170904085104-e3b1f968fc63 // indirect
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20210208195552-ff826a37aa15 // indirect
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d // indirect
	github.com/bifurcation/mint v0.0.0-20181105071958-a14404e9a861 // indirect
	github.com/caddyserver/caddy v1.0.3 // indirect
	github.com/cenkalti/backoff v2.1.1+incompatible // indirect
	github.com/cheekybits/genny v1.0.0 // indirect
	github.com/coreos/go-iptables v0.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.0-20190314233015-f79a8a8ca69d // indirect
	github.com/creack/pty v1.1.13 // indirect
	github.com/dchest/siphash v1.2.0 // indirect
	github.com/disintegration/imaging v1.6.2 // indirect
	github.com/docker/libcontainer v2.2.1+incompatible // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/dsoprea/go-exif/v3 v3.0.0-20201216222538-db167117f483 // indirect
	github.com/dsoprea/go-logging v0.0.0-20200517223158-a10564966e9d // indirect
	github.com/dsoprea/go-utility/v2 v2.0.0-20200717064901-2fccff4aa15e // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.1 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/flynn/go-shlex v0.0.0-20150515145356-3f9db97f8568 // indirect
	github.com/ginuerzh/gosocks4 v0.0.1 // indirect
	github.com/ginuerzh/gosocks5 v0.2.0 // indirect
	github.com/ginuerzh/tls-dissector v0.0.2-0.20200224064855-24ab2b3a3796 // indirect
	github.com/go-acme/lego v2.5.0+incompatible // indirect
	github.com/go-errors/errors v1.1.1 // indirect
	github.com/go-gost/relay v0.1.0 // indirect
	github.com/go-log/log v0.2.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang/geo v0.0.0-20200319012246-673a6f80352d // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/klauspost/compress v1.11.12 // indirect
	github.com/klauspost/cpuid v1.2.0 // indirect
	github.com/klauspost/cpuid/v2 v2.0.2 // indirect
	github.com/klauspost/reedsolomon v1.9.12 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/labstack/gommon v0.3.0 // indirect
	github.com/lucas-clemente/aes12 v0.0.0-20171027163421-cd47fb39b79f // indirect
	github.com/lucas-clemente/quic-go v0.10.2 // indirect
	github.com/lucas-clemente/quic-go-certificates v0.0.0-20160823095156-d2f86524cced // indirect
	github.com/maruel/natural v0.0.0-20180416170133-dbcb3e2e8cf1 // indirect
	github.com/marusama/semaphore/v2 v2.4.1 // indirect
	github.com/mattn/go-colorable v0.1.2 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mholt/archiver v3.1.1+incompatible // indirect
	github.com/mholt/certmagic v0.6.2-0.20190624175158-6a42ef9fe8c2 // indirect
	github.com/miekg/dns v1.1.41 // indirect
	github.com/milosgajdos/tenus v0.0.3 // indirect
	github.com/myesui/uuid v1.0.0 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/nwaples/rardecode v1.0.0 // indirect
	github.com/onsi/ginkgo v1.16.3 // indirect
	github.com/onsi/gomega v1.13.0 // indirect
	github.com/pierrec/lz4 v0.0.0-20190131084431-473cd7ce01a1 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/riobard/go-bloom v0.0.0-20200614022211-cdc8013cb5b3 // indirect
	github.com/russross/blackfriday/v2 v2.0.1 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/shadowsocks/go-shadowsocks2 v0.1.4 // indirect
	github.com/shadowsocks/shadowsocks-go v0.0.0-20170121203516-97a5c71f80ba // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8 // indirect
	github.com/templexxx/cpufeat v0.0.0-20180724012125-cef66df7f161 // indirect
	github.com/templexxx/xor v0.0.0-20191217153810-f85b25db303b // indirect
	github.com/tjfoc/gmsm v1.4.0 // indirect
	github.com/tomasen/realip v0.0.0-20180522021738-f0c99a92ddce // indirect
	github.com/ulikunitz/xz v0.5.6 // indirect
	github.com/urfave/cli v1.22.5 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.1 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/xtaci/kcp-go v5.4.20+incompatible // indirect
	github.com/xtaci/lossyconn v0.0.0-20200209145036-adba10fffc37 // indirect
	github.com/xtaci/smux v1.5.15 // indirect
	github.com/xtaci/tcpraw v1.2.25 // indirect
	github.com/yudai/hcl v0.0.0-00010101000000-000000000001 // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	golang.org/x/image v0.0.0-20191009234506-e7c1f5e7dbb8 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	gopkg.in/square/go-jose.v2 v2.2.2 // indirect
	gopkg.in/stretchr/testify.v1 v1.2.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	modernc.org/gc/v3 v3.0.0-20240107210532-573471604cb6 // indirect
	modernc.org/libc v1.55.3 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
	modernc.org/strutil v1.2.0 // indirect
	modernc.org/token v1.1.0 // indirect
)
