(function () {
    function randomColor() {
        // Generate a random but valid HEX color value
        let color = '#'
        for (let i = 0; i < 3; i++) {
            color += Math.floor(Math.random() * 256).toString(16)
        }
        return color
    }

    function loadMap() {
        // Get the map element
        let mapEl = document.getElementById('map')

        // Read the map data
        // Map page
        let locations = !mapEl.dataset.locations ? [] : JSON.parse(mapEl.dataset.locations)
        let tracks = !mapEl.dataset.tracks ? [] : JSON.parse(mapEl.dataset.tracks)
        // Post map
        let paths = !mapEl.dataset.paths ? [] : JSON.parse(mapEl.dataset.paths)
        let points = !mapEl.dataset.points ? [] : JSON.parse(mapEl.dataset.points)

        // Create Leaflet map
        let map = L.map('map', {
            minZoom: mapEl.dataset.minzoom,
            maxZoom: mapEl.dataset.maxzoom
        })

        // Set tile source and attribution
        L.tileLayer("/-/tiles/{s}/{z}/{x}/{y}.png", {
            attribution: mapEl.dataset.attribution,
        }).addTo(map)

        // Add features to the map
        let features = []
        locations.forEach(loc => {
            features.push(L.marker([loc.Lat, loc.Lon]).addTo(map).on('click', function () {
                window.open(loc.Post, '_blank').focus()
            }))
        })
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
        paths.forEach(path => {
            features.push(L.polyline(path.map(point => [point.Lat, point.Lon]), { color: 'blue' }).addTo(map))
        })
        points.forEach(point => {
            features.push(L.marker([point.Lat, point.Lon]).addTo(map))
        })

        // Make the map fit the features
        map.fitBounds(L.featureGroup(features).getBounds(), { padding: [5, 5] })
    }

    // Add Leaflet to the page

    // CSS
    let css = document.createElement('link')
    css.rel = 'stylesheet'
    css.href = '/-/leaflet/leaflet.css?v=1.8.0'
    document.head.appendChild(css)

    // JS
    let script = document.createElement('script')
    script.src = '/-/leaflet/leaflet.js?v=1.8.0'
    script.onload = loadMap
    document.head.appendChild(script)
})()