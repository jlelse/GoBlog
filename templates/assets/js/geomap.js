(function () {
    let mapEl = document.getElementById('map')
    let locations = JSON.parse(mapEl.dataset.locations)

    let map = L.map('map')

    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
    }).addTo(map)

    let markers = []
    locations.forEach(loc => {
        let marker = [loc.Lat, loc.Lon]
        L.marker(marker).addTo(map).on('click', function () {
            window.open(loc.Post, '_blank').focus()
        })
        markers.push(marker)
    })

    map.fitBounds(markers)
    map.zoomOut(2, { animate: false })
})()