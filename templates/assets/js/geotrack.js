(function () {
    let mapEl = document.getElementById('map')
    let paths = JSON.parse(mapEl.dataset.paths)

    let map = L.map('map')

    L.tileLayer("/x/tiles/{z}/{x}/{y}.png", {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
    }).addTo(map)

    let polylines = []
    paths.forEach(path => {
        let pathPoints = []
        path.forEach(point => {
            pathPoints.push([point.Lat, point.Lon])
        })
        let pl = L.polyline(pathPoints, { color: 'blue' }).addTo(map)
        polylines.push(pl)
    })
    let fgb = L.featureGroup(polylines).getBounds()
    map.fitBounds(fgb, { padding: [5, 5] })
})()