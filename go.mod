module go.goblog.app/app

go 1.19

require (
	git.jlel.se/jlelse/go-geouri v0.0.0-20210525190615-a9c1d50f42d6
	git.jlel.se/jlelse/go-shutdowner v0.0.0-20210707065515-773db8099c30
	git.jlel.se/jlelse/goldmark-mark v0.0.0-20210522162520-9788c89266a4
	git.jlel.se/jlelse/template-strings v0.0.0-20220211095702-c012e3b5045b
	github.com/PuerkitoBio/goquery v1.8.0
	github.com/alecthomas/chroma/v2 v2.3.0
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de
	github.com/c2h5oh/datasize v0.0.0-20220606134207-859f65c6625b
	github.com/carlmjohnson/requests v0.22.3
	github.com/cretz/bine v0.2.0
	github.com/dchest/captcha v1.0.0
	github.com/dgraph-io/ristretto v0.1.0
	github.com/disintegration/imaging v1.6.2
	github.com/dmulholl/mp3lib v1.0.0
	github.com/elnormous/contenttype v1.0.3
	github.com/emersion/go-sasl v0.0.0-20220912192320-0145f2c60ead
	github.com/emersion/go-smtp v0.15.0
	github.com/go-chi/chi/v5 v5.0.7
	github.com/go-fed/httpsig v1.1.0
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/google/uuid v1.3.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/sessions v1.2.1
	github.com/gorilla/websocket v1.5.0
	github.com/hacdias/indieauth/v2 v2.1.0
	github.com/jlaffaye/ftp v0.1.0
	// master
	github.com/jlelse/feeds v1.2.1-0.20210704161900-189f94254ad4
	github.com/justinas/alice v1.2.0
	github.com/kaorimatz/go-opml v0.0.0-20210201121027-bc8e2852d7f9
	github.com/klauspost/compress v1.15.11
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
	github.com/lopezator/migrator v0.3.1
	github.com/mattn/go-sqlite3 v1.14.15
	github.com/mergestat/timediff v0.0.3
	github.com/microcosm-cc/bluemonday v1.0.21
	github.com/mmcdole/gofeed v1.1.3
	github.com/paulmach/go.geojson v1.4.0
	github.com/posener/wstest v1.2.0
	github.com/pquerna/otp v1.3.0
	github.com/samber/lo v1.29.0
	github.com/schollz/sqlite3dump v1.3.1
	github.com/snabb/sitemap v1.0.0
	github.com/spf13/cast v1.5.0
	github.com/spf13/viper v1.13.0
	github.com/stretchr/testify v1.8.0
	github.com/tdewolff/minify/v2 v2.12.3
	// master
	github.com/tkrajina/gpxgo v1.2.2-0.20220217201249-321f19554eec
	github.com/tomnomnom/linkheader v0.0.0-20180905144013-02ca5825eb80
	github.com/traefik/yaegi v0.14.2
	github.com/vcraescu/go-paginator v1.0.1-0.20201114172518-2cfc59fe05c2
	github.com/yuin/goldmark v1.5.2
	// master
	github.com/yuin/goldmark-emoji v1.0.2-0.20210607094911-0487583eca38
	golang.org/x/crypto v0.0.0-20220722155217-630584e8d5aa
	golang.org/x/net v0.0.0-20221002022538-bcab6841153b
	golang.org/x/sync v0.0.0-20220929204114-8fcdb60fdcc0
	golang.org/x/text v0.3.7
	gopkg.in/yaml.v3 v3.0.1
	nhooyr.io/websocket v1.8.7
	tailscale.com v1.30.2
	// main
	willnorris.com/go/microformats v1.1.2-0.20210827044458-ff2a6ae41971
)

require (
	filippo.io/edwards25519 v1.0.0-rc.1 // indirect
	github.com/akutz/memconn v0.1.0 // indirect
	github.com/alexbrainman/sspi v0.0.0-20210105120005-909beea2cc74 // indirect
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/aws/aws-sdk-go-v2 v1.11.2 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.11.0 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.6.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.8.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.0.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.5.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssm v1.17.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.6.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.11.1 // indirect
	github.com/aws/smithy-go v1.9.0 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/boombuler/barcode v1.0.1-0.20190219062509-6c824513bacc // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/coreos/go-iptables v0.6.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dlclark/regexp2 v1.4.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.1 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/fxamacker/cbor/v2 v2.4.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/godbus/dbus/v5 v5.0.6 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/gorilla/securecookie v1.1.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hdevalence/ed25519consensus v0.0.0-20220222234857-c00d1f31bab3 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20211209223715-7d93572ebe8e // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/josharian/native v1.0.0 // indirect
	github.com/jsimonetti/rtnetlink v1.1.2-0.20220408201609-d380b505068b // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kortschak/wol v0.0.0-20200729010619-da482cc4850a // indirect
	github.com/lestrrat-go/strftime v1.0.5 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/mdlayher/genetlink v1.2.0 // indirect
	github.com/mdlayher/netlink v1.6.0 // indirect
	github.com/mdlayher/sdnotify v1.0.0 // indirect
	github.com/mdlayher/socket v0.2.3 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mmcdole/goxpp v0.0.0-20181012175147-0068e33feabf // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pelletier/go-toml/v2 v2.0.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/snabb/diagio v1.0.0 // indirect
	github.com/spf13/afero v1.8.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.4.1 // indirect
	github.com/tailscale/certstore v0.1.1-0.20220316223106-78d6e1c49d8d // indirect
	github.com/tailscale/golang-x-crypto v0.0.0-20220428210705-0b941c09a5e1 // indirect
	github.com/tailscale/goupnp v1.0.1-0.20210804011211-c64d0f06ea05 // indirect
	github.com/tailscale/netlink v1.1.1-0.20211101221916-cabfb018fe85 // indirect
	github.com/tcnksm/go-httpstat v0.2.0 // indirect
	github.com/tdewolff/parse/v2 v2.6.4 // indirect
	github.com/u-root/uio v0.0.0-20220204230159-dac05f7d2cb4 // indirect
	github.com/vishvananda/netlink v1.1.1-0.20211118161826-650dca95af54 // indirect
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go4.org/mem v0.0.0-20210711025021-927187094b94 // indirect
	go4.org/netipx v0.0.0-20220725152314-7e7bdc8411bf // indirect
	golang.org/x/exp v0.0.0-20220722155223-a9213eeb770e // indirect
	golang.org/x/image v0.0.0-20220413100746-70e8d0d3baa9 // indirect
	golang.org/x/oauth2 v0.0.0-20220411215720-9780585627b5 // indirect
	golang.org/x/sys v0.0.0-20220728004956-3c1f35247d10 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/time v0.0.0-20211116232009-f0f3c7e86c11 // indirect
	golang.zx2c4.com/wintun v0.0.0-20211104114900-415007cec224 // indirect
	golang.zx2c4.com/wireguard v0.0.0-20220703234212-c31a7b1ab478 // indirect
	golang.zx2c4.com/wireguard/windows v0.4.10 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gvisor.dev/gvisor v0.0.0-20220801230058-850e42eb4444 // indirect
	willnorris.com/go/webmention v0.0.0-20220108183051-4a23794272f0 // indirect
)
