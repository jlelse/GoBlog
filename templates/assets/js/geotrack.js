(function () {
    let mapEl = document.getElementById('map')
    let paths = JSON.parse(mapEl.dataset.paths)

    let map = L.map('map')

    L.tileLayer("/x/tiles/{z}/{x}/{y}.png", {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
    }).addTo(map)

    let polylines = []

    paths.forEach(path => {
        polylines.push(L.polyline(path.map(point => [point.Lat, point.Lon]), { color: 'blue' }).addTo(map))
    })

    map.fitBounds(L.featureGroup(polylines).getBounds(), { padding: [5, 5] })
})()