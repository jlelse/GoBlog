(function () {
    let mapEl = document.getElementById('map')
    let locations = (mapEl.dataset.locations == "") ? [] : JSON.parse(mapEl.dataset.locations)
    let tracks = (mapEl.dataset.tracks == "") ? [] : JSON.parse(mapEl.dataset.tracks)

    let map = L.map('map')

    L.tileLayer("/x/tiles/{z}/{x}/{y}.png", {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
    }).addTo(map)

    let features = []

    locations.forEach(loc => {
        features.push(L.marker([loc.Lat, loc.Lon]).addTo(map).on('click', function () {
            window.open(loc.Post, '_blank').focus()
        }))
    })

    tracks.forEach(track => {
        track.Paths.forEach(path => {
            features.push(L.polyline(path.map(point => [point.Lat, point.Lon]), { color: 'blue' }).addTo(map).on('click', function () {
                window.open(track.Post, '_blank').focus()
            }))
        })
        track.Points.forEach(point => {
            features.push(L.marker([point.Lat, point.Lon]).addTo(map).on('click', function () {
                window.open(track.Post, '_blank').focus()
            }))
        })
    })

    map.fitBounds(L.featureGroup(features).getBounds(), { padding: [5, 5] })
})()