module go.goblog.app/app

go 1.17

require (
	git.jlel.se/jlelse/go-geouri v0.0.0-20210525190615-a9c1d50f42d6
	git.jlel.se/jlelse/go-shutdowner v0.0.0-20210707065515-773db8099c30
	git.jlel.se/jlelse/goldmark-mark v0.0.0-20210522162520-9788c89266a4
	git.jlel.se/jlelse/template-strings v0.0.0-20210617205924-cfa3bd35ae40
	github.com/PuerkitoBio/goquery v1.8.0
	github.com/alecthomas/chroma v0.9.4
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de
	github.com/c2h5oh/datasize v0.0.0-20200825124411-48ed595a09d2
	github.com/cretz/bine v0.2.0
	github.com/dchest/captcha v0.0.0-20200903113550-03f5f0333e1f
	github.com/dgraph-io/ristretto v0.1.0
	github.com/dmulholl/mp3lib v1.0.0
	github.com/elnormous/contenttype v1.0.0
	github.com/emersion/go-sasl v0.0.0-20211008083017-0b9dcfb154ac
	github.com/emersion/go-smtp v0.15.0
	github.com/go-chi/chi/v5 v5.0.6
	github.com/go-fed/httpsig v1.1.0
	github.com/google/uuid v1.3.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v1.2.1
	github.com/gorilla/websocket v1.4.2
	github.com/jlaffaye/ftp v0.0.0-20211117213618-11820403398b
	// master
	github.com/jlelse/feeds v1.2.1-0.20210704161900-189f94254ad4
	github.com/justinas/alice v1.2.0
	github.com/kaorimatz/go-opml v0.0.0-20210201121027-bc8e2852d7f9
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
	github.com/lopezator/migrator v0.3.0
	github.com/mattn/go-sqlite3 v1.14.9
	github.com/microcosm-cc/bluemonday v1.0.16
	github.com/paulmach/go.geojson v1.4.0
	github.com/posener/wstest v1.2.0
	github.com/pquerna/otp v1.3.0
	github.com/schollz/sqlite3dump v1.3.1
	github.com/snabb/sitemap v1.0.0
	github.com/spf13/cast v1.4.1
	github.com/spf13/viper v1.9.0
	github.com/stretchr/testify v1.7.0
	github.com/tdewolff/minify/v2 v2.9.22
	github.com/thoas/go-funk v0.9.1
	github.com/tkrajina/gpxgo v1.1.2
	github.com/tomnomnom/linkheader v0.0.0-20180905144013-02ca5825eb80
	github.com/vcraescu/go-paginator v1.0.1-0.20201114172518-2cfc59fe05c2
	github.com/yuin/goldmark v1.4.4
	// master
	github.com/yuin/goldmark-emoji v1.0.2-0.20210607094911-0487583eca38
	github.com/yuin/goldmark-highlighting v0.0.0-20210516132338-9216f9c5aa01
	golang.org/x/crypto v0.0.0-20211117183948-ae814b36b871
	golang.org/x/net v0.0.0-20211116231205-47ca1ff31462
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/text v0.3.7
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	tailscale.com v1.16.2
	// main
	willnorris.com/go/microformats v1.1.2-0.20210827044458-ff2a6ae41971
)

// Override some modules with own forks
replace github.com/yuin/goldmark-highlighting => github.com/jlelse/goldmark-highlighting v0.0.0-20211115195757-39f0fea96680

require (
	github.com/alexbrainman/sspi v0.0.0-20210105120005-909beea2cc74 // indirect
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/boombuler/barcode v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/coreos/go-iptables v0.6.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dlclark/regexp2 v1.4.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-multierror/multierror v1.0.2 // indirect
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/godbus/dbus/v5 v5.0.5 // indirect
	github.com/golang/glog v1.0.0 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20210621130208-1cac67f12b1e // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/josharian/native v0.0.0-20200817173448-b6b71def0850 // indirect
	github.com/jsimonetti/rtnetlink v0.0.0-20210525051524-4cc836578190 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/lestrrat-go/strftime v1.0.5 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mdlayher/netlink v1.4.1 // indirect
	github.com/mdlayher/sdnotify v0.0.0-20210228150836-ea3ec207d697 // indirect
	github.com/mdlayher/socket v0.0.0-20210307095302-262dc9984e00 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/snabb/diagio v1.0.0 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/tailscale/certstore v0.0.0-20210528134328-066c94b793d3 // indirect
	github.com/tailscale/goupnp v1.0.1-0.20210804011211-c64d0f06ea05 // indirect
	github.com/tcnksm/go-httpstat v0.2.0 // indirect
	github.com/tdewolff/parse/v2 v2.5.22 // indirect
	github.com/u-root/uio v0.0.0-20210528114334-82958018845c // indirect
	go4.org/intern v0.0.0-20210108033219-3eb7198706b2 // indirect
	go4.org/mem v0.0.0-20201119185036-c04c5a6ff174 // indirect
	go4.org/unsafe/assume-no-moving-gc v0.0.0-20201222180813-1025295fd063 // indirect
	golang.org/x/sys v0.0.0-20211109184856-51b60fd695b3 // indirect
	golang.org/x/time v0.0.0-20210611083556-38a9dc6acbc6 // indirect
	golang.zx2c4.com/wireguard v0.0.0-20210905140043-2ef39d47540c // indirect
	golang.zx2c4.com/wireguard/windows v0.4.10 // indirect
	gopkg.in/ini.v1 v1.63.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	inet.af/netaddr v0.0.0-20210721214506-ce7a8ad02cc1 // indirect
	inet.af/netstack v0.0.0-20210622165351-29b14ebc044e // indirect
)
