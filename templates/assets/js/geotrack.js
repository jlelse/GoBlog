(function () {
    let mapEl = document.getElementById('map')
    let paths = (mapEl.dataset.paths == "") ? [] : JSON.parse(mapEl.dataset.paths)
    let points = (mapEl.dataset.points == "") ? [] : JSON.parse(mapEl.dataset.points)

    let map = L.map('map')

    L.tileLayer("/x/tiles/{z}/{x}/{y}.png", {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
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