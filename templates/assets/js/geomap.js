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

        let features = []
        function fitFeatures() {
            // Make the map fit the features
            map.fitBounds(L.featureGroup(features).getBounds(), { padding: [5, 5] })
        }

        // Map page
        getMapJson(mapEl.dataset.locations, locations => {
            locations.forEach(loc => {
                features.push(L.marker([loc.Lat, loc.Lon]).addTo(map).on('click', function () {
                    window.open(loc.Post, '_blank').focus()
                }))
            })
            fitFeatures()
        })
        getMapJson(mapEl.dataset.tracks, tracks => {
            tracks.forEach(track => {
                track.Paths.forEach(path => {
                    // Use random color on map page for paths to better differentiate
                    features.push(L.polyline(path.map(point => [point.Lat, point.Lon]), { color: randomColor() }).addTo(map).on('click', function () {
                        window.open(track.Post, '_blank').focus()
                    }))
                })
                track.Points.forEach(point => {
                    features.push(L.marker([point.Lat, point.Lon]).addTo(map).on('click', function () {
                        window.open(track.Post, '_blank').focus()
                    }))
                })
            })
            fitFeatures()
        })
        // Post map
        getMapJson(mapEl.dataset.paths, paths => {
            paths.forEach(path => {
                features.push(L.polyline(path.map(point => [point.Lat, point.Lon]), { color: 'blue' }).addTo(map))
            })
            fitFeatures()
        })
        getMapJson(mapEl.dataset.points, points => {
            points.forEach(point => {
                features.push(L.marker([point.Lat, point.Lon]).addTo(map))
            })
            fitFeatures()
        })

    }

    // Add Leaflet to the page

    // CSS
    let css = document.createElement('link')
    css.rel = 'stylesheet'
    css.href = '/-/leaflet/leaflet.css?v=1.9.2'
    document.head.appendChild(css)

    // JS
    let script = document.createElement('script')
    script.src = '/-/leaflet/leaflet.js?v=1.9.2'
    script.onload = loadMap
    document.head.appendChild(script)
})()