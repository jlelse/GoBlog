(function () {
    function randomColor() {
        // Generate one of 30 different colors
        return `hsl(${Math.floor(Math.random() * 30) * 12}, 100%, 50%)`
    }

    function parseMapJson(json) {
        try {
            return JSON.parse(json)
        } catch (error) {
            console.error('Failed to parse map data', error);
            return undefined
        }
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
                    const parsed = parseMapJson(req.responseText)
                    if (parsed && parsed.length > 0) {
                        callback(parsed)
                    }
                }
            }
            req.send()
            return
        } else {
            const parsed = parseMapJson(data)
            if (parsed && parsed.length > 0) {
                callback(parsed)
            }
            return
        }
    }

    function openPost(post) {
        if (post) window.open(post, '_blank', 'noopener')
    }

    function loadStylesheet(href) {
        const link = document.createElement('link')
        link.rel = 'stylesheet'
        link.href = href
        document.head.appendChild(link)
    }

    function loadMapFeatures(mapEl, addPoint, addLine, onDataLoaded) {
        getMapJson(mapEl.dataset.locations, locations => {
            locations.forEach(location => addPoint(location.Point, location.Post))
            onDataLoaded()
        })
        getMapJson(mapEl.dataset.tracks, tracks => {
            tracks.forEach(track => {
                (track.Paths || []).forEach(path => addLine(path, track.Post));
                (track.Points || []).forEach(point => addPoint(point, track.Post))
            })
            onDataLoaded()
        })
        getMapJson(mapEl.dataset.paths, paths => {
            paths.forEach(path => addLine(path))
            onDataLoaded()
        })
        getMapJson(mapEl.dataset.points, points => {
            points.forEach(point => addPoint(point))
            onDataLoaded()
        })
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

        const addPoint = (point, post) => {
            const marker = L.marker(point)
            if (post) marker.on('click', () => openPost(post))
            cluster.addLayer(marker)
        }
        const addLine = (path, post) => {
            const line = L.polyline(path, { color: randomColor() })
            if (post) line.on('click', () => openPost(post))
            cluster.addLayer(line)
        }
        loadMapFeatures(mapEl, addPoint, addLine, fitFeatures)
    }

    function loadVectorMap() {
        const mapEl = document.getElementById('map')
        maplibregl.setWorkerUrl('/-/maplibre/maplibre-gl-worker.js?v=5.6.2')
        const map = new maplibregl.Map({
            container: 'map',
            style: '/-/maptiles/styles/bright',
            minZoom: Number(mapEl.dataset.minzoom),
            maxZoom: Number(mapEl.dataset.maxzoom),
            boxZoom: false,
            attributionControl: false
        })
        const bounds = new maplibregl.LngLatBounds()
        let hasBounds = false
        const features = []
        const updateFeatures = () => map.getSource('goblog-features').setData({
            type: 'FeatureCollection', features: features
        })
        const addMarker = (point, post) => {
            const lngLat = [point[1], point[0]]
            features.push({ type: 'Feature', properties: { post: post || '' }, geometry: {
                type: 'Point', coordinates: lngLat
            } })
            bounds.extend(lngLat)
            hasBounds = true
        }
        const addLine = (path, post) => {
            if (!path || path.length < 2) return
            const coordinates = path.map(point => {
                const lngLat = [point[1], point[0]]
                bounds.extend(lngLat)
                hasBounds = true
                return lngLat
            })
            features.push({ type: 'Feature', properties: { color: randomColor(), post: post || '' }, geometry: {
                type: 'LineString', coordinates: coordinates
            } })
        }
        map.on('load', () => {
            map.addSource('goblog-features', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } })
            map.addLayer({ id: 'goblog-tracks', type: 'line', source: 'goblog-features', filter: ['==', '$type', 'LineString'], layout: {
                'line-cap': 'round', 'line-join': 'round'
            }, paint: {
                'line-color': ['get', 'color'], 'line-width': 3
            } })
            map.addLayer({ id: 'goblog-points', type: 'circle', source: 'goblog-features', filter: ['==', '$type', 'Point'], paint: {
                'circle-color': '#1769aa', 'circle-radius': 6, 'circle-stroke-color': '#fff', 'circle-stroke-width': 2
            } })
            const interactiveLayers = ['goblog-points', 'goblog-tracks']
            map.on('click', interactiveLayers, event => {
                openPost(event.features?.[0]?.properties?.post)
            })
            map.on('mouseenter', interactiveLayers, event => {
                if (event.features?.some(f => f.properties?.post)) {
                    mapEl.classList.add('goblog-map-hover')
                }
            })
            map.on('mouseleave', interactiveLayers, () => {
                mapEl.classList.remove('goblog-map-hover')
            })
            const onDataLoaded = () => {
                updateFeatures()
                if (hasBounds) map.fitBounds(bounds, { padding: 24, maxZoom: 15, duration: 0 })
            }
            loadMapFeatures(mapEl, addMarker, addLine, onDataLoaded)
        })
    }

    if (document.getElementById('map')?.dataset.vector === 'true') {
        loadStylesheet('/-/maplibre/maplibre-gl.css?v=5.6.2')
        loadStylesheet('/-/maplibre/goblog.css')
        const script = document.createElement('script')
        script.src = '/-/maplibre/maplibre-gl.js?v=5.6.2'
        script.integrity = 'sha256-9meVnU1AsmIFUyAYqE0g7KRvj8nB1rH+c2nWoW74QJc='
        script.onload = loadVectorMap
        document.head.appendChild(script)
        return
    }

    // Add Leaflet to the page

    // CSS
    loadStylesheet('/-/leaflet/leaflet.css?v=1.9.4')
    loadStylesheet('/-/leaflet/markercluster.css?v=1.5.3')
    loadStylesheet('/-/leaflet/markercluster.default.css?v=1.5.3')

    // JS
    const script = document.createElement('script')
    script.src = '/-/leaflet/leaflet.js?v=1.9.4'
    script.integrity = 'sha256-3HH4pogLw8ob2fqNxfGvSMcC3FELCngkCgfF/u186TU='
    script.onload = function () {
        // Marker Cluster plugin
        const plugin = document.createElement('script')
        plugin.src = '/-/leaflet/markercluster.js?v=1.5.3'
        plugin.integrity = 'sha256-se80zO4Ps22Fb/RxnHWQRqDhQpPbv51THZaXNUgRaLw='
        plugin.onload = loadMap
        document.head.appendChild(plugin)
    }
    document.head.appendChild(script)
})()