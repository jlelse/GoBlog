(function () {
    let mapEl = document.getElementById('map')
    let paths = (mapEl.dataset.paths == "") ? [] : JSON.parse(mapEl.dataset.paths)
    let points = (mapEl.dataset.points == "") ? [] : JSON.parse(mapEl.dataset.points)

    let map = L.map('map', {
        minZoom: mapEl.dataset.minzoom,
        maxZoom: mapEl.dataset.maxzoom
    })

    L.tileLayer("/-/tiles/{s}/{z}/{x}/{y}.png", {
        attribution: mapEl.dataset.attribution,
    }).addTo(map)

    let features = []

    paths.forEach(path => {
        features.push(L.polyline(path.map(point => [point.Lat, point.Lon]), { color: 'blue' }).addTo(map))
    })

    points.forEach(point => {
        features.push(L.marker([point.Lat, point.Lon]).addTo(map))
    })

    map.fitBounds(L.featureGroup(features).getBounds(), { padding: [5, 5] })
})()