(function () {
    function randomColor() {
        // Generate one of 30 different colors
        return `hsl(${Math.floor(Math.random() * 30) * 12}, 100%, 50%)`
    }

    function getMapJson(data, callback) {
        if (!data) {
            return
        } else if (data.startsWith('url:')) {
            const url = data.substring(4)
            const req = new XMLHttpRequest()
            req.open('GET', url)
            req.onload = function () {
                if (req.status == 200) {
                    const parsed = JSON.parse(req.responseText)
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
        const mapEl = document.getElementById('map')

        // Create Leaflet map
        const map = L.map('map', {
            minZoom: mapEl.dataset.minzoom,
            maxZoom: mapEl.dataset.maxzoom
        })

        // Set tile source and attribution
        L.tileLayer("/-/tiles/{s}/{z}/{x}/{y}.png", {
            attribution: mapEl.dataset.attribution,
        }).addTo(map)

        // Load map features
        const cluster = L.markerClusterGroup().addTo(map)

        function fitFeatures() {
            // Make the map fit the features
            map.fitBounds(cluster.getBounds(), { padding: [5, 5] })
        }

        // Map page
        getMapJson(mapEl.dataset.locations, locations => {
            locations.forEach(location => {
                cluster.addLayer(L.marker(location.Point).on('click', function () {
                    window.open(location.Post, '_blank').focus()
                }))
            })
            fitFeatures()
        })
        getMapJson(mapEl.dataset.tracks, tracks => {
            tracks.forEach(track => {
                track.Paths.forEach(path => {
                    // Use random color on map page for paths to better differentiate
                    cluster.addLayer(L.polyline(path, { color: randomColor() }).on('click', function () {
                        window.open(track.Post, '_blank').focus()
                    }))
                })
                track.Points.forEach(point => {
                    cluster.addLayer(L.marker(point).on('click', function () {
                        window.open(track.Post, '_blank').focus()
                    }))
                })
            })
            fitFeatures()
        })
        // Post map
        getMapJson(mapEl.dataset.paths, paths => {
            paths.forEach(path => {
                cluster.addLayer(L.polyline(path, { color: randomColor() }))
            })
            fitFeatures()
        })
        getMapJson(mapEl.dataset.points, points => {
            points.forEach(point => {
                cluster.addLayer(L.marker(point))
            })
            fitFeatures()
        })

    }

    // Add Leaflet to the page

    // CSS
    const css = document.createElement('link')
    css.rel = 'stylesheet'
    css.href = '/-/leaflet/leaflet.css?v=1.9.4'
    document.head.appendChild(css)

    // Marker Cluster plugin
    const pluginCss1 = document.createElement('link')
    pluginCss1.rel = 'stylesheet'
    pluginCss1.href = '/-/leaflet/markercluster.css?v=1.5.3'
    document.head.appendChild(pluginCss1)

    const pluginCss2 = document.createElement('link')
    pluginCss2.rel = 'stylesheet'
    pluginCss2.href = '/-/leaflet/markercluster.default.css?v=1.5.3'
    document.head.appendChild(pluginCss2)

    // JS
    const script = document.createElement('script')
    script.src = '/-/leaflet/leaflet.js?v=1.9.4'
    script.onload = function () {
        // Marker Cluster plugin
        const plugin = document.createElement('script')
        plugin.src = '/-/leaflet/markercluster.js?v=1.5.3'
        plugin.onload = loadMap
        document.head.appendChild(plugin)
    }
    document.head.appendChild(script)
})()