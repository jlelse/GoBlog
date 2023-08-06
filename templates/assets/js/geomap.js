(function () {
    function randomColor() {
        let color = '#'
        for (let i = 0; i < 3; i++) {
            color += Math.floor(10 + Math.random() * 246).toString(16)
        }
        return color
    }

    function getMapJson(data, callback) {
        if (!data) {
            return
        } else if (data.startsWith('url:')) {
            let url = data.substring(4)
            let req = new XMLHttpRequest()
            req.open('GET', url)
            req.onload = function () {
                if (req.status == 200) {
                    let parsed = JSON.parse(req.responseText)
                    if (parsed && parsed.length > 0) {
                        callback(parsed)
                    }
                }
            }
            req.send()
            return
        } else {
            callback(JSON.parse(data))
            return
        }
    }

    function loadMap() {
        // Get the map element
        let mapEl = document.getElementById('map')

        // Create Leaflet map
        let map = L.map('map', {
            minZoom: mapEl.dataset.minzoom,
            maxZoom: mapEl.dataset.maxzoom
        })

        // Set tile source and attribution
        L.tileLayer("/-/tiles/{s}/{z}/{x}/{y}.png", {
            attribution: mapEl.dataset.attribution,
        }).addTo(map)

        // Load map features
        let cluster = L.markerClusterGroup().addTo(map)

        function fitFeatures() {
            // Make the map fit the features
            map.fitBounds(cluster.getBounds(), { padding: [5, 5] })
        }

        // Map page
        getMapJson(mapEl.dataset.locations, locations => {
            locations.forEach(loc => {
                cluster.addLayer(L.marker([loc.Lat, loc.Lon]).on('click', function () {
                    window.open(loc.Post, '_blank').focus()
                }))
            })
            fitFeatures()
        })
        getMapJson(mapEl.dataset.tracks, tracks => {
            tracks.forEach(track => {
                track.Paths.forEach(path => {
                    // Use random color on map page for paths to better differentiate
                    cluster.addLayer(L.polyline(path.map(point => [point.Lat, point.Lon]), { color: randomColor() }).on('click', function () {
                        window.open(track.Post, '_blank').focus()
                    }))
                })
                track.Points.forEach(point => {
                    cluster.addLayer(L.marker([point.Lat, point.Lon]).on('click', function () {
                        window.open(track.Post, '_blank').focus()
                    }))
                })
            })
            fitFeatures()
        })
        // Post map
        getMapJson(mapEl.dataset.paths, paths => {
            paths.forEach(path => {
                cluster.addLayer(L.polyline(path.map(point => [point.Lat, point.Lon]), { color: 'blue' }))
            })
            fitFeatures()
        })
        getMapJson(mapEl.dataset.points, points => {
            points.forEach(point => {
                cluster.addLayer(L.marker([point.Lat, point.Lon]))
            })
            fitFeatures()
        })

    }

    // Add Leaflet to the page

    // CSS
    let css = document.createElement('link')
    css.rel = 'stylesheet'
    css.href = '/-/leaflet/leaflet.css?v=1.9.4'
    document.head.appendChild(css)

    // Marker Cluster plugin
    let pluginCss1 = document.createElement('link')
    pluginCss1.rel = 'stylesheet'
    pluginCss1.href = '/-/leaflet/markercluster.css?v=1.5.3'
    document.head.appendChild(pluginCss1)

    let pluginCss2 = document.createElement('link')
    pluginCss2.rel = 'stylesheet'
    pluginCss2.href = '/-/leaflet/markercluster.default.css?v=1.5.3'
    document.head.appendChild(pluginCss2)

    // JS
    let script = document.createElement('script')
    script.src = '/-/leaflet/leaflet.js?v=1.9.4'
    script.onload = function () {
        // Marker Cluster plugin
        let plugin = document.createElement('script')
        plugin.src = '/-/leaflet/markercluster.js?v=1.5.3'
        plugin.onload = loadMap
        document.head.appendChild(plugin)
    }
    document.head.appendChild(script)
})()